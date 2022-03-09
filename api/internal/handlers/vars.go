package handlers

import (
	"bufio"
	"bytes"
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"encoding/json"
	"encoding/pem"
	"errors"
	"io/ioutil"
	"main/internal"
	"net/http"
	"os"
	"os/exec"
	"strconv"
	"strings"
	"time"
)

func dumpHeader(r *http.Request) string {
	headerBytes, _ := json.Marshal(r.Header)
	return string(headerBytes)
}

type clusterCfg struct {
	//required inputs
	Region string `json:"REGION"` //us-east-1
	Domain string `json:"DOMAIN"` //myhubs.net

	//required? but possible to fallback to locally available values
	Env                     string `json:"env"`                     //dev
	OAUTH_CLIENT_ID_FXA     string `json:"OAUTH_CLIENT_ID_FXA"`     //2db93e6523568888
	OAUTH_CLIENT_SECRET_FXA string `json:"OAUTH_CLIENT_SECRET_FXA"` //06e08133333333333387dd5425234388ac4e29999999999905a2eaea7e1d8888
	SMTP_SERVER             string `json:"SMTP_SERVER"`             //email-smtp.us-east-1.amazonaws.com
	SMTP_PORT               string `json:"SMTP_PORT"`               //25
	SMTP_USER               string `json:"SMTP_USER"`               //AKIAYEJRSWRAQUI7U3J4
	SMTP_PASS               string `json:"SMTP_PASS"`               //BL+rv9q1noXMNWB4D8re8DUGQ7dPXlL6aq5cqod18UFC
	AWS_KEY                 string `json:"AWS_KEY"`                 //AKIAYEJRSWRAQSAM8888
	AWS_SECRET              string `json:"AWS_SECRET"`              //AKIAYEJRSWRAQSAM8888AKIAYEJRSWRAQSAM8888
	// this will just be Region ...
	AWS_REGION string `json:"AWS_REGION"` //us-east-1

	//optional inputs
	CF_DeploymentPrefix  string `json:"name"`                 //t-
	CF_deploymentId      string `json:"cf_deploymentId"`      //s0meid
	AWS_Ingress_Cert_ARN string `json:"aws_ingress_cert_arn"` //arn:aws:acm:us-east-1:123456605633:certificate/123456ab-f861-470b-a837-123456a76e17
	Options              string `json:"options"`              //additional options, dot(.)prefixed -- ie. ".dryrun"

	//generated pre-infra-deploy
	CF_Stackname  string `json:"stackname"`
	DB_PASS       string `json:"DB_PASS"`       //itjfHE8888
	COOKIE_SECRET string `json:"COOKIE_SECRET"` //a-random-string-to-sign-auth-cookies
	PERMS_KEY     string `json:"PERMS_KEY"`     //-----BEGIN RSA PRIVATE KEY-----\\nMIIEpgIBAAKCAQEA3RY0qLmdthY6Q0RZ4oyNQSL035BmYLNdleX1qVpG1zfQeLWf\\n/otgc8Ho2w8y5wW2W5vpI4a0aexNV2evgfsZKtx0q5WWwjsr2xy0Ak1zhWTgZD+F\\noHVGJ0xeFse2PnEhrtWalLacTza5RKEJskbNiTTu4fD+UfOCMctlwudNSs+AkmiP\\nSxc8nWrZ5BuvdnEXcJOuw0h4oyyUlkmj+Oa/ZQVH44lmPI9Ih0OakXWpIfOob3X0\\nXqcdywlMVI2hzBR3JNodRjyEz33p6E//lY4Iodw9NdcRpohGcxcgQ5vf4r4epLIa\\ncr0y5w1ZiRyf6BwyqJ6IBpA7yYpws3r9qxmAqwIDAQABAoIBAQCgwy/hbK9wo3MU\\nTNRrdzaTob6b/l1jfanUgRYEYl/WyYAu9ir0JhcptVwERmYGNVIoBRQfQClaSHjo\\n0L1/b74aO5oe1rR8Yhh+yL1gWz9gRT0hyEr7paswkkhsmiY7+3m5rxsrfinlM+6+\\nJ7dsSi3U0ofOBbZ4kvAeEz/Y3OaIOUbQraP312hQnTVQ3kp7HNi9GcLK9rq2mASu\\nO0DxDHXdZMsRN1K4tOKRZDsKGAEfL2jKN7+ndvsDhb4mAQaVKM8iw+g5O4HDA8uB\\nmwycaWhjilZWEyUyqvXE8tOMLS59sq6i1qrf8zIMWDOizebF/wnrQ42kzt5kQ0ZJ\\nwCPOC3sxAoGBAO6KfWr6WsXD6phnjVXXi+1j3azRKJGQorwQ6K3bXmISdlahngas\\nmBGBmI7jYTrPPeXAHUbARo/zLcbuGCf1sPipkAHYVC8f9aUbA205BREB15jNyXr3\\nXzhR/ronbn0VeR9iRua2FZjVChz22fdz9MvRJiinP8agYIQ4LovDk3lzAoGBAO1E\\nrZpOuv3TMQffPaPemWuvMYfZLgx2/AklgYqSoi683vid9HEEAdVzNWMRrOg0w5EH\\nWMEMPwJTYvy3xIgcFmezk5RMHTX2J32JzDJ8Y/uGf1wMrdkt3LkPRfuGepEDDtBa\\nrUSO/MeGXLu5p8QByUZkvTLJ4rJwF2HZBUehrm3pAoGBANg1+tveNCyRGbAuG/M0\\nvgXbwO+FXWojWP1xrhT3gyMNbOm079FI20Ty3F6XRmfRtF7stRyN5udPGaz33jlJ\\n/rBEsNybQiK8qyCNzZtQVYFG1C4SSI8GbO5Vk7cTSphhwDlsEKvJWuX+I36BWKts\\nFPQwjI/ImIvmjdUKP1Y7XQ51AoGBALWa5Y3ASRvStCqkUlfFH4TuuWiTcM2VnN+b\\nV4WrKnu/kKKWs+x09rpbzjcf5kptaGrvRp2sM+Yh0RhByCmt5fBF4OWXRJxy5lMO\\nT78supJgpcbc5YvfsJvs9tHIYrPvtT0AyrI5B33od74wIhrCiz5YCQCAygVuCleY\\ndpQXSp1RAoGBAKjasot7y/ErVxq7LIpGgoH+XTxjvMsj1JwlMeK0g3sjnun4g4oI\\nPBtpER9QaSFi2OeYPklJ2g2yvFcVzj/pFk/n1Zd9pWnbU+JIXBYaHTjmktLeZHsb\\nrTEKATo+Y1Alrhpr/z7gXXDfuKKXHkVRiper1YRAxELoLJB8r7LWeuIb\\n-----END RSA PRIVATE KEY-----
	//generated post-infra-deploy
	DB_HOST string `json:"DB_HOST"` //geng-test4turkey-db.ccgehrnbveo1.us-east-1.rds.amazonaws.com
	DB_CONN string `json:"DB_CONN"` //postgres://postgres:itjfHE8888@geng-test4turkey-db.ccgehrnbveo1.us-east-1.rds.amazonaws.com
	PSQL    string `json:"PSQL"`    //postgresql://postgres:itjfHE8888@geng-test4turkey-db.ccgehrnbveo1.us-east-1.rds.amazonaws.com/ret_dev
}

func turkey_makeCfg(r *http.Request, sess *internal.CacheBoxSessData) (clusterCfg, error) {
	var cfg clusterCfg

	//get r.body
	rBodyBytes, err := ioutil.ReadAll(r.Body)
	if err != nil {
		internal.GetLogger().Warn("ERROR @ reading r.body, error = " + err.Error())
		return cfg, err
	}
	//make cfg
	err = json.Unmarshal(rBodyBytes, &cfg)
	if err != nil {
		internal.GetLogger().Warn("bad clusterCfg: " + string(rBodyBytes))
		return cfg, err
	}

	//required inputs
	if cfg.Region == "" {
		return cfg, errors.New("bad input: Region is required")
	}
	cfg.AWS_REGION = cfg.Region

	if cfg.Domain == "" {
		return cfg, errors.New("bad input: Domain is required")
	}
	//required but with fallbacks
	if cfg.OAUTH_CLIENT_ID_FXA == "" {
		internal.GetLogger().Warn("OAUTH_CLIENT_ID_FXA not supplied, falling back to local value")
		cfg.OAUTH_CLIENT_ID_FXA = os.Getenv("OAUTH_CLIENT_ID_FXA")
	}
	if cfg.OAUTH_CLIENT_SECRET_FXA == "" {
		internal.GetLogger().Warn("OAUTH_CLIENT_SECRET_FXA not supplied, falling back to local value")
		cfg.OAUTH_CLIENT_SECRET_FXA = os.Getenv("OAUTH_CLIENT_SECRET_FXA")
	}
	if cfg.SMTP_SERVER == "" {
		internal.GetLogger().Warn("SMTP_SERVER not supplied, falling back to local value")
		cfg.SMTP_SERVER = internal.Cfg.SmtpServer
	}
	if cfg.SMTP_PORT == "" {
		internal.GetLogger().Warn("SMTP_PORT not supplied, falling back to local value")
		cfg.SMTP_PORT = internal.Cfg.SmtpPort
	}
	if cfg.SMTP_USER == "" {
		internal.GetLogger().Warn("SMTP_USER not supplied, falling back to local value")
		cfg.SMTP_USER = internal.Cfg.SmtpUser
	}
	if cfg.SMTP_PASS == "" {
		internal.GetLogger().Warn("SMTP_PASS not supplied, falling back to local value")
		cfg.SMTP_PASS = internal.Cfg.SmtpPass
	}
	if cfg.AWS_KEY == "" {
		internal.GetLogger().Warn("AWS_KEY not supplied, falling back to local value")
		cfg.AWS_KEY = internal.Cfg.AwsKey
	}
	if cfg.AWS_SECRET == "" {
		internal.GetLogger().Warn("AWS_SECRET not supplied, falling back to local value")
		cfg.AWS_SECRET = internal.Cfg.AwsSecret
	}
	if cfg.Env == "" {
		cfg.Env = "dev"
		internal.GetLogger().Warn("Env unspecified -- using dev")
	}

	//optional inputs
	if cfg.CF_DeploymentPrefix == "" {
		cfg.CF_DeploymentPrefix = "t-"
		internal.GetLogger().Warn("CF_DeploymentPrefix unspecified -- using (default)" + cfg.CF_DeploymentPrefix)
	}
	if cfg.CF_deploymentId == "" {
		cfg.CF_deploymentId = strconv.FormatInt(time.Now().Unix()-1626102245, 36)
		internal.GetLogger().Info("CF_deploymentId: " + cfg.CF_deploymentId)
	}
	cfg.CF_Stackname = cfg.CF_DeploymentPrefix + cfg.CF_deploymentId

	//generate the rest
	cfg.DB_PASS = internal.PwdGen(15)
	cfg.COOKIE_SECRET = internal.PwdGen(15)
	cfg.DB_HOST = "to-be-determined-after-infra-deployment"
	cfg.DB_CONN = "to-be-determined-after-infra-deployment"
	cfg.PSQL = "to-be-determined-after-infra-deployment"
	var pvtKey, _ = rsa.GenerateKey(rand.Reader, 2048)
	pvtKeyBytes := x509.MarshalPKCS1PrivateKey(pvtKey)
	pemBytes := pem.EncodeToMemory(&pem.Block{Type: "RSA PRIVATE KEY", Bytes: pvtKeyBytes})
	pemString := string(pemBytes)
	cfg.PERMS_KEY = strings.ReplaceAll(pemString, "\n", `\\n`)

	return cfg, nil
}

func reportCmd(cmd *exec.Cmd) error {
	var out bytes.Buffer
	var stderr bytes.Buffer
	cmd.Stdout = &out
	cmd.Stderr = &stderr

	// err := cmd.Run()
	// if err != nil {
	// 	return errors.New("[ERR] cmd failed -- " + fmt.Sprint(err) + ": " + stderr.String())
	// }

	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return err
	}

	err = cmd.Start()
	internal.GetLogger().Debug("running: " + cmd.String())
	if err != nil {
		return err
	}

	// print the output of the subprocess
	scanner := bufio.NewScanner(stdout)
	for scanner.Scan() {
		m := scanner.Text()
		internal.GetLogger().Debug(m)
	}
	cmd.Wait()

	return nil
}
