package internal

import (
	"bytes"
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"encoding/pem"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"io/ioutil"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/service/s3"
	"github.com/square/go-jose"
)

func DeployKeys(awss *AwsSvs, stackName, targetBucket string) error {

	tmpWorkDir := "./tmp_" + stackName
	err := MakeKeys(stackName, tmpWorkDir)
	if err != nil {
		panic(err)
	}
	//wait for bucket
	err = awss.S3WaitForBucket(targetBucket, 600)
	if err != nil {
		panic(err)
	}

	// files, err := ioutil.ReadDir(tmpWorkDir)
	// if err != nil {
	// 	panic(err)
	// }
	// for _, file := range files {
	// 	f, err := os.Open(tmpWorkDir + "/" + file.Name())
	// 	if err != nil {
	// 		panic(err)
	// 	}
	// 	awss.S3UploadFile(f, targetBucket, "/keys/"+file.Name())
	// }

	err = filepath.Walk(tmpWorkDir,
		func(path string, info fs.FileInfo, err error) error {
			if err != nil {
				return err
			}
			if info.IsDir() {
				return nil
			}
			f, err := os.Open(path)
			if err != nil {
				panic(err)
			}
			s3Key := "/keys/" + info.Name()
			if strings.Contains(path, "/bio/") {
				s3Key = "/keys/bio/" + info.Name()
			}
			awss.S3UploadFile(f, targetBucket, s3Key)

			return nil
		})
	if err != nil {
		panic(err)
	}

	return nil
}

func DeployHubsAssets(awss *AwsSvs, metaMap map[string]string, turkeycfgBucket, targetBucket string) {
	//wait for bucket
	err := awss.S3WaitForBucket(targetBucket, 600)
	if err != nil {
		panic(err)
	}

	s3svc := s3.New(awss.Sess)
	metaStr := hubsCfg_metaMapToMetaString(metaMap)
	pageNum := 0
	err = s3svc.ListObjectsPages(
		&s3.ListObjectsInput{Bucket: aws.String(turkeycfgBucket), Prefix: aws.String("rawhubs/")},
		func(page *s3.ListObjectsOutput, lastPage bool) bool {
			pageNum++
			fmt.Println("pageNum", pageNum, "lastPage", lastPage)
			for _, s3obj := range page.Contents {
				key := *s3obj.Key
				targetKey := strings.Replace(key, `rawhubs/`, `hubs/`, 1)
				ext := filepath.Ext(key)
				isHtml := ext == ".html"
				isCss := ext == ".css"
				if isHtml || isCss {
					f, _ := ioutil.TempFile("./", "hubs-tmp-")
					awss.S3Download_file("turkeycfg", key, f)
					fBytes, _ := ioutil.ReadFile(f.Name())
					fStr := string(fBytes)
					fStr = hubsCfg_baseAssetsPath(fStr, metaMap["base_assets_path"])

					if isHtml {
						fStr = hubsCfg_meta(fStr, metaStr)
					}
					f.Truncate(0)
					f.Seek(0, io.SeekStart)
					f.WriteString(fStr)
					f.Seek(0, io.SeekStart)
					awss.S3UploadMimeObj(f, targetBucket, targetKey, GetMimeType(ext))
					f.Close()
					os.Remove(f.Name())
				} else {
					if ext == ".wasm" {
						s3svc.CopyObject((&s3.CopyObjectInput{
							Bucket:      aws.String(targetBucket),
							CopySource:  aws.String(url.PathEscape("turkeycfg/" + key)),
							Key:         aws.String(targetKey),
							ContentType: aws.String("application/wasm"),
						}))
					} else {
						s3svc.CopyObject((&s3.CopyObjectInput{
							Bucket:     aws.String(targetBucket),
							CopySource: aws.String(url.PathEscape("turkeycfg/" + key)),
							Key:        aws.String(targetKey),
						}))
					}
				}
			}
			return true
		})
	if err != nil {
		panic("s3svc.ListObjectsPages failed: %q" + err.Error())
	}
}

func hubsCfg_baseAssetsPath(in, baseAssetPath string) string {
	r := strings.ReplaceAll(in, "{{rawhubs-base-assets-path}}/", baseAssetPath)
	r = strings.ReplaceAll(r, "{{rawhubs-base-assets-path}}", baseAssetPath)
	return r
}

func hubsCfg_metaMapToMetaString(metaMap map[string]string) string {
	r := "\n"
	for k := range metaMap {
		r += `<meta name="env:` + k + `" content="` + metaMap[k] + `"/>` + "\n"
	}
	return r
}

func hubsCfg_meta(in, meta string) string {
	metaAnchor := `<!-- DO NOT REMOVE/EDIT THIS COMMENT - META_TAGS -->`
	return strings.ReplaceAll(in, metaAnchor, meta+metaAnchor)
}

func reportCmd(cmd *exec.Cmd) error {
	var out bytes.Buffer
	var stderr bytes.Buffer
	cmd.Stdout = &out
	cmd.Stderr = &stderr
	err := cmd.Run()
	if err != nil {
		return errors.New("[ERR] cmd failed -- " + fmt.Sprint(err) + ": " + stderr.String())
	}
	// fmt.Println("Result: " + out.String())
	return nil
}

func MakeKeys(stackName, workDir string) error {
	var err error

	cmd_bio := exec.Command("./_tools/bio", "--version")
	err = reportCmd(cmd_bio)
	if err != nil {
		panic(err)
	}

	//bio ring key
	cmd_ringKey := exec.Command("./_tools/bio", "ring", "key", "generate", stackName, "--cache-key-path", workDir+"/bio")
	err = reportCmd(cmd_ringKey)
	if err != nil {
		panic(err)
	}
	//bio svc keys
	for _, svcName := range []string{
		"reticulum.default",
		"janus-gateway.default",
		"postgrest.default",
		"coturn.default",
		"certbot.default",
		"ita.default",
		"speelycaptor.default",
		"photomnemonic.default",
		"youtube-dl-api-server.default",
		"pgbouncer.default",
		"imgproxy.default",
		"hubs.default",
		"spoke.default",
		"polycosm-static-assets.default",
	} {
		cmd_svcKey := exec.Command("./_tools/bio", "svc", "key", "generate", svcName, stackName, "--cache-key-path", workDir+"/bio")
		err := reportCmd(cmd_svcKey)
		if err != nil {
			panic(err)
		}
	}
	//bio user keys
	userName := "polycosm-config-user"
	cmd_usrKey := exec.Command("./_tools/bio", "user", "key", "generate", userName, "--cache-key-path", workDir+"/bio")
	err = reportCmd(cmd_usrKey)
	if err != nil {
		panic(err)
	}
	//ssl keys
	var pvtKey, _ = rsa.GenerateKey(rand.Reader, 2048)
	pvtKey.Precompute()
	pvtKeyBytes := x509.MarshalPKCS1PrivateKey(pvtKey)
	ioutil.WriteFile(workDir+"/jwt-key.der", pvtKeyBytes, 0600)
	ioutil.WriteFile(workDir+"/jwt-key.pem", pem.EncodeToMemory(&pem.Block{Type: "RSA PRIVATE KEY", Bytes: pvtKeyBytes}), 0600)
	pubKeyBytes, _ := x509.MarshalPKIXPublicKey(pvtKey.PublicKey)
	ioutil.WriteFile(workDir+"/jwt-pub.der", pubKeyBytes, 0600)
	ioutil.WriteFile(workDir+"/jwt-pub.pem", pem.EncodeToMemory(&pem.Block{Type: "RSA PUBLIC KEY", Bytes: pubKeyBytes}), 0600)
	//jwk json
	jwk := jose.JSONWebKey{Key: pvtKey}
	jwkJsonBytes, _ := jwk.MarshalJSON()
	ioutil.WriteFile(workDir+"/jwt-pub.json", jwkJsonBytes, 0600)

	//
	// cmd_ls := exec.Command("ls", "-lha", workDir)
	// err = reportCmd(cmd_ls)
	// if err != nil {
	// 	panic(err)
	// }

	//drop the timestamp string in bio key file names
	files, err := ioutil.ReadDir(workDir)
	if err != nil {
		panic(err)
	}
	reg, _ := regexp.Compile(`\-\d{14}\.`)
	for _, file := range files {
		oldName := file.Name()
		newName := reg.ReplaceAllString(file.Name(), "")
		os.Rename(filepath.Join(workDir, oldName), filepath.Join(workDir, newName))
	}

	//
	// cmd_ls2 := exec.Command("ls", "-lha", workDir)
	// err = reportCmd(cmd_ls2)
	// if err != nil {
	// 	panic(err)
	// }

	return nil
}
