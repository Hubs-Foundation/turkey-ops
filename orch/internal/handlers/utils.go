package handlers

import (
	"crypto"
	"crypto/ecdsa"
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"encoding/base64"
	"encoding/json"
	"encoding/pem"
	"errors"
	"fmt"
	"hash/fnv"
	"io/ioutil"
	"main/internal"
	"math/big"
	"net/http"
	"os"
	"strconv"
	"strings"
	"sync/atomic"
	"text/template"
	"time"
)

func pointerOfInt32(i int) *int32 {
	int32i := int32(i)
	return &int32i
}

// var KeepAlive = http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {

// })

var Dummy = http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {

	// perms_key_str_in := `-----BEGIN RSA PRIVATE KEY-----\\nMIIEpgIBAAKCAQEA3RY0qLmdthY6Q0RZ4oyNQSL035BmYLNdleX1qVpG1zfQeLWf\\n/otgc8Ho2w8y5wW2W5vpI4a0aexNV2evgfsZKtx0q5WWwjsr2xy0Ak1zhWTgZD+F\\noHVGJ0xeFse2PnEhrtWalLacTza5RKEJskbNiTTu4fD+UfOCMctlwudNSs+AkmiP\\nSxc8nWrZ5BuvdnEXcJOuw0h4oyyUlkmj+Oa/ZQVH44lmPI9Ih0OakXWpIfOob3X0\\nXqcdywlMVI2hzBR3JNodRjyEz33p6E//lY4Iodw9NdcRpohGcxcgQ5vf4r4epLIa\\ncr0y5w1ZiRyf6BwyqJ6IBpA7yYpws3r9qxmAqwIDAQABAoIBAQCgwy/hbK9wo3MU\\nTNRrdzaTob6b/l1jfanUgRYEYl/WyYAu9ir0JhcptVwERmYGNVIoBRQfQClaSHjo\\n0L1/b74aO5oe1rR8Yhh+yL1gWz9gRT0hyEr7paswkkhsmiY7+3m5rxsrfinlM+6+\\nJ7dsSi3U0ofOBbZ4kvAeEz/Y3OaIOUbQraP312hQnTVQ3kp7HNi9GcLK9rq2mASu\\nO0DxDHXdZMsRN1K4tOKRZDsKGAEfL2jKN7+ndvsDhb4mAQaVKM8iw+g5O4HDA8uB\\nmwycaWhjilZWEyUyqvXE8tOMLS59sq6i1qrf8zIMWDOizebF/wnrQ42kzt5kQ0ZJ\\nwCPOC3sxAoGBAO6KfWr6WsXD6phnjVXXi+1j3azRKJGQorwQ6K3bXmISdlahngas\\nmBGBmI7jYTrPPeXAHUbARo/zLcbuGCf1sPipkAHYVC8f9aUbA205BREB15jNyXr3\\nXzhR/ronbn0VeR9iRua2FZjVChz22fdz9MvRJiinP8agYIQ4LovDk3lzAoGBAO1E\\nrZpOuv3TMQffPaPemWuvMYfZLgx2/AklgYqSoi683vid9HEEAdVzNWMRrOg0w5EH\\nWMEMPwJTYvy3xIgcFmezk5RMHTX2J32JzDJ8Y/uGf1wMrdkt3LkPRfuGepEDDtBa\\nrUSO/MeGXLu5p8QByUZkvTLJ4rJwF2HZBUehrm3pAoGBANg1+tveNCyRGbAuG/M0\\nvgXbwO+FXWojWP1xrhT3gyMNbOm079FI20Ty3F6XRmfRtF7stRyN5udPGaz33jlJ\\n/rBEsNybQiK8qyCNzZtQVYFG1C4SSI8GbO5Vk7cTSphhwDlsEKvJWuX+I36BWKts\\nFPQwjI/ImIvmjdUKP1Y7XQ51AoGBALWa5Y3ASRvStCqkUlfFH4TuuWiTcM2VnN+b\\nV4WrKnu/kKKWs+x09rpbzjcf5kptaGrvRp2sM+Yh0RhByCmt5fBF4OWXRJxy5lMO\\nT78supJgpcbc5YvfsJvs9tHIYrPvtT0AyrI5B33od74wIhrCiz5YCQCAygVuCleY\\ndpQXSp1RAoGBAKjasot7y/ErVxq7LIpGgoH+XTxjvMsj1JwlMeK0g3sjnun4g4oI\\nPBtpER9QaSFi2OeYPklJ2g2yvFcVzj/pFk/n1Zd9pWnbU+JIXBYaHTjmktLeZHsb\\nrTEKATo+Y1Alrhpr/z7gXXDfuKKXHkVRiper1YRAxELoLJB8r7LWeuIb\\n-----END RSA PRIVATE KEY-----`

	// perms_key_str := strings.Replace(perms_key_str_in, `\\n`, "\n", -1)
	// pb, _ := pem.Decode([]byte(perms_key_str))
	// perms_key, _ := x509.ParsePKCS1PrivateKey(pb.Bytes)
	// // var perms_key, _ = rsa.GenerateKey(rand.Reader, 2048)

	// pvtKeyBytes := x509.MarshalPKCS1PrivateKey(perms_key)
	// pvtKeyPemBytes := pem.EncodeToMemory(&pem.Block{Type: "RSA PRIVATE KEY", Bytes: pvtKeyBytes})

	// jwk, err := jwkEncode(perms_key.Public())
	// if err != nil {
	// 	panic(err)
	// }
	// fmt.Println(jwk)

	// json.NewEncoder(w).Encode(map[string]string{
	// 	"perms_key": strings.Replace(string(pvtKeyPemBytes), "\n", `\n`, -1),
	// 	"jwk":       jwk,
	// })
	//-------------------------------------------------

	// conn, err := internal.PgxPool.Acquire(context.Background())
	// if err != nil {
	// 	fmt.Fprintln(os.Stderr, "Error acquiring connection:", err)
	// 	os.Exit(1)
	// }

	// dbName := "ret_geng_test_3"
	// _, err = conn.Exec(context.Background(), "create database "+dbName)
	// if err != nil {
	// 	panic(err)
	// }
	// retSchemaBytes, err := ioutil.ReadFile("./_files/pgSchema.sql")
	// if err != nil {
	// 	panic(err)
	// }
	// dbconn, err := pgx.Connect(context.Background(), internal.Cfg.DBconn+"/"+dbName)
	// if err != nil {
	// 	panic(err)
	// }
	// _, err = dbconn.Exec(context.Background(), string(retSchemaBytes))
	// if err != nil {
	// 	panic(err)
	// }
	// dbconn.Close(context.Background())

	// fmt.Println(" ~~~ hello from /Dummy ~~~ ~~~ ~~~ dumping r !!!")

})

func Healthz() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if atomic.LoadInt32(&internal.Healthy) == 1 {
			w.WriteHeader(http.StatusNoContent)
			return
		}
		w.WriteHeader(http.StatusServiceUnavailable)
	})
}

// jwkEncode encodes public part of an RSA or ECDSA key into a JWK.
// The result is also suitable for creating a JWK thumbprint.
// https://tools.ietf.org/html/rfc7517
func jwkEncode(pub crypto.PublicKey) (string, error) {
	switch pub := pub.(type) {
	case *rsa.PublicKey:
		// https://tools.ietf.org/html/rfc7518#section-6.3.1
		n := pub.N
		e := big.NewInt(int64(pub.E))
		// Field order is important.
		// See https://tools.ietf.org/html/rfc7638#section-3.3 for details.
		return fmt.Sprintf(`{"e":"%s","kty":"RSA","n":"%s"}`,
			base64.RawURLEncoding.EncodeToString(e.Bytes()),
			base64.RawURLEncoding.EncodeToString(n.Bytes()),
		), nil
	case *ecdsa.PublicKey:
		// https://tools.ietf.org/html/rfc7518#section-6.2.1
		p := pub.Curve.Params()
		n := p.BitSize / 8
		if p.BitSize%8 != 0 {
			n++
		}
		x := pub.X.Bytes()
		if n > len(x) {
			x = append(make([]byte, n-len(x)), x...)
		}
		y := pub.Y.Bytes()
		if n > len(y) {
			y = append(make([]byte, n-len(y)), y...)
		}
		// Field order is important.
		// See https://tools.ietf.org/html/rfc7638#section-3.3 for details.
		return fmt.Sprintf(`{"crv":"%s","kty":"EC","x":"%s","y":"%s"}`,
			p.Name,
			base64.RawURLEncoding.EncodeToString(x),
			base64.RawURLEncoding.EncodeToString(y),
		), nil
	}
	return "", errors.New("bad key")
}

type clusterCfg struct {
	//required inputs
	Region string `json:"region"` //us-east-1
	Domain string `json:"domain"` //myhubs.net

	//required? but possible to fallback to locally available values
	Env                     string `json:"env"`                     //dev
	OAUTH_CLIENT_ID_FXA     string `json:"OAUTH_CLIENT_ID_FXA"`     //2db93e6523568888
	OAUTH_CLIENT_SECRET_FXA string `json:"OAUTH_CLIENT_SECRET_FXA"` //06e08133333333333387dd5425234388ac4e29999999999905a2eaea7e1d8888
	SMTP_SERVER             string `json:"SMTP_SERVER"`             //email-smtp.us-east-1.amazonaws.com
	SMTP_PORT               string `json:"SMTP_PORT"`               //25
	SMTP_USER               string `json:"SMTP_USER"`               //AKIAYEJRSWRAQUI7U3J4
	SMTP_PASS               string `json:"SMTP_PASS"`               //BL+rv9q1noXMNWB4D8re8DUGQ7dPXlL6aq5cqod18UFC
	GCP_SA_KEY_b64          string `json:"GCP_SA_KEY"`              // cat $(the gcp-iam-service-account-key-json file)
	AWS_KEY                 string `json:"AWS_KEY"`                 //AKIAYEJRSWRAQSAM8888
	AWS_SECRET              string `json:"AWS_SECRET"`              //AKIAYEJRSWRAQSAM8888AKIAYEJRSWRAQSAM8888
	GCP_SA_HMAC_KEY         string `json:"GCP_SA_HMAC_KEY"`         //https://cloud.google.com/storage/docs/authentication/hmackeys, ie.GOOG1EGPHPZU7F3GUTJCVQWLTYCY747EUAVHHEHQBN4WXSMPXJU4488888888
	GCP_SA_HMAC_SECRET      string `json:"GCP_SA_HMAC_SECRET"`      //https://cloud.google.com/storage/docs/authentication/hmackeys, ie.0EWCp6g4j+MXn32RzOZ8eugSS5c0fydT88888888
	SKETCHFAB_API_KEY       string `json:"SKETCHFAB_API_KEY"`
	// this will just be Region ...
	ItaChan string `json:"itachan"` //ita's listening channel (dev, beta, stable), falls back to Env, swaping staging/prod for beta/stable
	CLOUD   string `json:"cloud"`   //aws or gcp or azure or something else like nope or local etc

	//optional inputs
	DeploymentPrefix     string `json:"name"`                 //t-
	DeploymentId         string `json:"deploymentId"`         //s0meid
	AWS_Ingress_Cert_ARN string `json:"aws_ingress_cert_arn"` //arn:aws:acm:us-east-1:123456605633:certificate/123456ab-f861-470b-a837-123456a76e17
	Options              string `json:"options"`              //additional options, dot(.)prefixed -- ie. ".dryrun"

	//generated pre-infra-deploy
	Stackname            string `json:"stackname"`
	DB_USER              string `json:"DB_USER"`       //postgres
	DB_PASS              string `json:"DB_PASS"`       //itjfHE8888
	COOKIE_SECRET        string `json:"COOKIE_SECRET"` //a-random-string-to-sign-auth-cookies
	PERMS_KEY            string `json:"PERMS_KEY"`     //-----BEGIN RSA PRIVATE KEY-----\\nMIIEpgIBA...AKCAr7LWeuIb\\n-----END RSA PRIVATE KEY-----
	PERMS_KEY_PUB_b64    string `json:"PERMS_KEY_PUB_b64"`
	DASHBOARD_ACCESS_KEY string `json:"DASHBOARD_ACCESS_KEY"` // api key for DASHBOARD access

	//initiated pre-infra-deploy, generated post-infra-deploy
	DB_HOST string `json:"DB_HOST"` //geng-test4turkey-db.ccgehrnbveo1.us-east-1.rds.amazonaws.com
	DB_CONN string `json:"DB_CONN"` //postgres://postgres:itjfHE8888@geng-test4turkey-db.ccgehrnbveo1.us-east-1.rds.amazonaws.com
	PSQL    string `json:"PSQL"`    //postgresql://postgres:itjfHE8888@geng-test4turkey-db.ccgehrnbveo1.us-east-1.rds.amazonaws.com/ret_dev
}

func turkey_getCfg(r *http.Request) (clusterCfg, error) {
	var cfg clusterCfg
	//get r.body
	rBodyBytes, err := ioutil.ReadAll(r.Body)
	if err != nil {
		internal.Logger.Warn("ERROR @ reading r.body, error = " + err.Error())
		return cfg, err
	}
	//unmarshal to cfg
	err = json.Unmarshal(rBodyBytes, &cfg)
	if err != nil {
		internal.Logger.Warn("bad clusterCfg: " + string(rBodyBytes))
		return cfg, err
	}
	return cfg, nil
}

// load current cfg from stack to populate ommited fields in inputedCfg
func turkey_loadStackCfg(stackname string, inputedCfg clusterCfg) (clusterCfg, error) {
	var cfg clusterCfg
	//get cfg.json from turkeycfg bucket
	cfgBytes, err := internal.Cfg.Gcps.GCS_ReadFile("turkeycfg", "tf-backend/"+stackname+"/cfg.json")
	if err != nil {
		return cfg, nil
	}
	//unmarshal to cfg
	err = json.Unmarshal(cfgBytes, &cfg)
	if err != nil {
		internal.Logger.Warn("bad clusterCfg: " + string(cfgBytes))
		return cfg, err
	}
	// for ommited files in inputedCfg -- load from (previous deployed) cfg
	cfg_m, err := clusterCfgToMap(cfg)
	if err != nil {
		return cfg, err
	}
	inputedCfg_m, err := clusterCfgToMap(inputedCfg)
	if err != nil {
		return cfg, err
	}
	for k, v := range inputedCfg_m {
		if v == "" {
			inputedCfg_m[k] = cfg_m[k]
		}
	}
	var loadedCfg clusterCfg
	loadedCfgJsonByte, err := json.Marshal(inputedCfg_m)
	if err != nil {
		return cfg, err
	}
	err = json.Unmarshal(loadedCfgJsonByte, &loadedCfg)
	if err != nil {
		internal.Logger.Error("failed to Unmarshal loadedCfgJsonByte " + string(loadedCfgJsonByte))
		return loadedCfg, err
	}
	return loadedCfg, nil
}

func clusterCfgToMap(cfg clusterCfg) (map[string]string, error) {
	var m map[string]string
	cfgJsonByte, err := json.Marshal(cfg)
	if err != nil {
		return nil, err
	}
	err = json.Unmarshal(cfgJsonByte, &m)
	if err != nil {
		internal.Logger.Error("failed to Unmarshal: " + string(cfgJsonByte))
		return nil, err
	}
	return m, nil
}

func turkey_makeCfg(r *http.Request) (clusterCfg, error) {

	cfg, err := turkey_getCfg(r)
	if err != nil {
		return cfg, err
	}
	if cfg.Stackname != "" {
		internal.Logger.Debug("loading current cfg for: " + cfg.Stackname)
		cfg, err = turkey_loadStackCfg(cfg.Stackname, cfg)
		if err != nil {
			internal.Logger.Error("failed to load provided cluster stackname: " + cfg.Stackname)
			return cfg, err
		}
	}

	//required inputs
	if cfg.Region == "" {
		return cfg, errors.New("bad input: can't figure out value for Region")
	}

	// todo -- make a util func to use a regex to validate domain name
	if cfg.Domain == "" || strings.HasPrefix(cfg.Domain, "changeMe") {
		return cfg, errors.New("bad input: Domain is required")
	}

	//required but with fallbacks
	if cfg.OAUTH_CLIENT_ID_FXA == "" {
		internal.Logger.Warn("OAUTH_CLIENT_ID_FXA not supplied, falling back to local value")
		cfg.OAUTH_CLIENT_ID_FXA = os.Getenv("OAUTH_CLIENT_ID_FXA")
	}
	if cfg.OAUTH_CLIENT_SECRET_FXA == "" {
		internal.Logger.Warn("OAUTH_CLIENT_SECRET_FXA not supplied, falling back to local value")
		cfg.OAUTH_CLIENT_SECRET_FXA = os.Getenv("OAUTH_CLIENT_SECRET_FXA")
	}
	if cfg.SMTP_SERVER == "" {
		internal.Logger.Warn("SMTP_SERVER not supplied, falling back to local value")
		cfg.SMTP_SERVER = internal.Cfg.SmtpServer
	}
	if cfg.SMTP_PORT == "" {
		internal.Logger.Warn("SMTP_PORT not supplied, falling back to local value")
		cfg.SMTP_PORT = internal.Cfg.SmtpPort
	}
	if cfg.SMTP_USER == "" {
		internal.Logger.Warn("SMTP_USER not supplied, falling back to local value")
		cfg.SMTP_USER = internal.Cfg.SmtpUser
	}
	if cfg.SMTP_PASS == "" {
		internal.Logger.Warn("SMTP_PASS not supplied, falling back to local value")
		cfg.SMTP_PASS = internal.Cfg.SmtpPass
	}
	if cfg.AWS_KEY == "" {
		internal.Logger.Warn("AWS_KEY not supplied, falling back to local value")
		cfg.AWS_KEY = internal.Cfg.AwsKey
	}
	if cfg.AWS_SECRET == "" {
		internal.Logger.Warn("AWS_SECRET not supplied, falling back to local value")
		cfg.AWS_SECRET = internal.Cfg.AwsSecret
	}
	if cfg.GCP_SA_HMAC_KEY == "" {
		internal.Logger.Warn("GCP_SA_HMAC_KEY not supplied, falling back to local value")
		cfg.GCP_SA_HMAC_KEY = internal.Cfg.GCP_SA_HMAC_KEY
	}
	if cfg.GCP_SA_HMAC_SECRET == "" {
		internal.Logger.Warn("GCP_SA_HMAC_SECRET not supplied, falling back to local value")
		cfg.GCP_SA_HMAC_SECRET = internal.Cfg.GCP_SA_HMAC_SECRET
	}
	if cfg.SKETCHFAB_API_KEY == "" {
		internal.Logger.Warn("SKETCHFAB_API_KEY not supplied, falling back to local value")
		cfg.SKETCHFAB_API_KEY = internal.Cfg.SKETCHFAB_API_KEY
	}
	if cfg.Env == "" {
		cfg.Env = "dev"
		internal.Logger.Warn("Env unspecified -- using dev")
	}
	if cfg.ItaChan == "" {
		cfg.ItaChan = cfg.Env
		if cfg.ItaChan == "staging" {
			cfg.ItaChan = "beta"
		}
		if cfg.ItaChan == "prod" {
			cfg.ItaChan = "stable"
		}
		internal.Logger.Warn("ItaChan unspecified -- falling back to Env (swaping staging/prod for beta/stable): " + cfg.ItaChan)
	}

	//optional inputs
	if cfg.DeploymentPrefix == "" {
		cfg.DeploymentPrefix = strings.ReplaceAll(cfg.Domain, ".", "")
		internal.Logger.Warn("deploymentPrefix unspecified -- using (default)" + cfg.DeploymentPrefix)
	}
	if cfg.DeploymentId == "" {
		cfg.DeploymentId = strconv.FormatInt(time.Now().Unix()-1648672222, 36)
		internal.Logger.Info("deploymentId: " + cfg.DeploymentId)
	}
	if cfg.Stackname == "" {
		cfg.Stackname = cfg.DeploymentPrefix + cfg.DeploymentId
	}

	//generate the rest
	pwdSeed := int64(hash(cfg.Stackname))
	cfg.DB_USER = "postgres"
	cfg.DB_PASS = internal.PwdGen(15, pwdSeed, "D~")
	cfg.COOKIE_SECRET = internal.PwdGen(15, pwdSeed, "C~")
	cfg.DB_HOST = "to-be-determined-after-infra-deployment"
	cfg.DB_CONN = "to-be-determined-after-infra-deployment"
	cfg.PSQL = "to-be-determined-after-infra-deployment"
	var pvtKey, _ = rsa.GenerateKey(rand.Reader, 2048)
	pvtKeyBytes := x509.MarshalPKCS1PrivateKey(pvtKey)
	pemBytes := pem.EncodeToMemory(&pem.Block{Type: "RSA PRIVATE KEY", Bytes: pvtKeyBytes})
	cfg.PERMS_KEY = strings.ReplaceAll(string(pemBytes), "\n", `\\n`)
	pubKey := pvtKey.PublicKey
	pubKeyBytes := x509.MarshalPKCS1PublicKey(&pubKey)
	pubKey_pemBytes := pem.EncodeToMemory(&pem.Block{Type: "RSA PUBLIC KEY", Bytes: pubKeyBytes})
	// cfg.PERMS_KEY_PUB = strings.ReplaceAll(string(pubKey_pemBytes), "\n", `\\n`)
	cfg.PERMS_KEY_PUB_b64 = base64.StdEncoding.EncodeToString(pubKey_pemBytes) //string(pubKey_pemBytes)
	if cfg.CLOUD == "" {
		internal.Logger.Warn("cfg.CLOUD unspecified, falling back to (because it probably is): gcp")
		cfg.CLOUD = "gcp"
	}

	if cfg.GCP_SA_KEY_b64 == "" {
		cfg.GCP_SA_KEY_b64 = base64.StdEncoding.EncodeToString([]byte(os.Getenv("GCP_SA_KEY")))
		internal.Logger.Warn("GCP_SA_KEY_b64 unspecified -- using: " + cfg.GCP_SA_KEY_b64)
	}
	cfg.DASHBOARD_ACCESS_KEY = internal.PwdGen(15, pwdSeed, "P~")

	return cfg, nil
}

func hash(s string) uint32 {
	h := fnv.New32a()
	h.Write([]byte(s))
	return h.Sum32()
}

func runTf(cfg clusterCfg, verb string, flags ...string) (string, []string, error) {
	wd, _ := os.Getwd()
	// render the template.tf with cfg.Stackname into a Stackname named folder so that
	// 1. we can run terraform from that folder
	// 2. terraform will use a Stackname named folder in it's remote backend
	tfTemplateFile := wd + "/_files/tf/gcp.tf.gotemplate"
	if _, err := os.Stat(tfTemplateFile); errors.Is(err, os.ErrNotExist) {
		return "", nil, err
	}

	// tf_bin := wd + "/_files/tf/terraform"
	tf_bin := "terraform"
	tfdir := wd + "/_files/tf/" + cfg.Stackname
	os.Mkdir(tfdir, os.ModePerm)

	tfFile := tfdir + "/rendered.tf"
	t, err := template.ParseFiles(tfTemplateFile)
	if err != nil {
		return "", nil, err
	}
	f, _ := os.Create(tfFile)
	defer f.Close()

	t.Execute(f, struct{ ProjectId, Stackname, Region, DbUser, DbPass, Env string }{
		ProjectId: internal.Cfg.Gcps.ProjectId,
		Stackname: cfg.Stackname,
		Region:    cfg.Region,
		DbUser:    cfg.DB_USER,
		DbPass:    cfg.DB_PASS,
		Env:       cfg.Env,
	})
	tfBytes, _ := ioutil.ReadFile(tfFile)
	tfFileStr := string(tfBytes)

	err, tf_out_init := internal.RunCmd_sync(tf_bin, "-chdir="+tfdir, "init")
	if err != nil {
		return tfFileStr, nil, err
	}

	args := []string{"-chdir=" + tfdir, verb}
	for _, flag := range flags {
		args = append(args, flag)
	}
	// err, out_verb := internal.RunCmd_sync(tf_bin, "-chdir="+tfdir, verb, flags)
	err, tf_out_verb := internal.RunCmd_sync(tf_bin, args...)
	if err != nil {
		return "", nil, err
	}
	return tfFileStr, append(tf_out_init, tf_out_verb...), nil
}
