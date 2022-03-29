package handlers

import (
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"encoding/base64"
	"encoding/json"
	"encoding/pem"
	"errors"
	"hash/fnv"
	"io/ioutil"
	"main/internal"
	"net/http"
	"os"
	"strconv"
	"strings"
	"time"
)

func Dumpheader(r *http.Request) string {
	headerBytes, _ := json.Marshal(r.Header)
	return string(headerBytes)
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
	// this will just be Region ...
	AWS_REGION string `json:"AWS_REGION"` //us-east-1

	//optional inputs
	deploymentPrefix     string `json:"name"`                 //t-
	deploymentId         string `json:"deploymentId"`         //s0meid
	AWS_Ingress_Cert_ARN string `json:"aws_ingress_cert_arn"` //arn:aws:acm:us-east-1:123456605633:certificate/123456ab-f861-470b-a837-123456a76e17
	Options              string `json:"options"`              //additional options, dot(.)prefixed -- ie. ".dryrun"

	//generated pre-infra-deploy
	Stackname     string `json:"stackname"`
	DB_PASS       string `json:"DB_PASS"`       //itjfHE8888
	COOKIE_SECRET string `json:"COOKIE_SECRET"` //a-random-string-to-sign-auth-cookies
	PERMS_KEY     string `json:"PERMS_KEY"`     //-----BEGIN RSA PRIVATE KEY-----\\nMIIEpgIBA...AKCAr7LWeuIb\\n-----END RSA PRIVATE KEY-----
	CLOUD         string `json:"cloud"`         //aws or gcp or azure or something else like nope or local etc
	//initiated pre-infra-deploy, generated post-infra-deploy
	DB_HOST string `json:"DB_HOST"` //geng-test4turkey-db.ccgehrnbveo1.us-east-1.rds.amazonaws.com
	DB_CONN string `json:"DB_CONN"` //postgres://postgres:itjfHE8888@geng-test4turkey-db.ccgehrnbveo1.us-east-1.rds.amazonaws.com
	PSQL    string `json:"PSQL"`    //postgresql://postgres:itjfHE8888@geng-test4turkey-db.ccgehrnbveo1.us-east-1.rds.amazonaws.com/ret_dev
}

func turkey_makeCfg(r *http.Request) (clusterCfg, error) {
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
	if cfg.deploymentPrefix == "" {
		cfg.deploymentPrefix = "t-"
		internal.GetLogger().Warn("deploymentPrefix unspecified -- using (default)" + cfg.deploymentPrefix)
	}
	if cfg.deploymentId == "" {
		cfg.deploymentId = strconv.FormatInt(time.Now().Unix()-1626102245, 36)
		internal.GetLogger().Info("deploymentId: " + cfg.deploymentId)
	}
	if cfg.Stackname == "" {
		cfg.Stackname = cfg.deploymentPrefix + cfg.deploymentId
	}

	//generate the rest
	pwdSeed := int64(hash(cfg.Stackname))
	cfg.DB_PASS = internal.PwdGen(15, pwdSeed)
	cfg.COOKIE_SECRET = internal.PwdGen(15, pwdSeed)
	cfg.DB_HOST = "to-be-determined-after-infra-deployment"
	cfg.DB_CONN = "to-be-determined-after-infra-deployment"
	cfg.PSQL = "to-be-determined-after-infra-deployment"
	var pvtKey, _ = rsa.GenerateKey(rand.Reader, 2048)
	pvtKeyBytes := x509.MarshalPKCS1PrivateKey(pvtKey)
	pemBytes := pem.EncodeToMemory(&pem.Block{Type: "RSA PRIVATE KEY", Bytes: pvtKeyBytes})
	pemString := string(pemBytes)
	cfg.PERMS_KEY = strings.ReplaceAll(pemString, "\n", `\\n`)
	cfg.CLOUD = "???"

	if cfg.GCP_SA_KEY_b64 == "" {
		cfg.GCP_SA_KEY_b64 = base64.StdEncoding.EncodeToString([]byte(os.Getenv("GCP_SA_KEY")))
		internal.GetLogger().Warn("GCP_SA_KEY_b64 unspecified -- using: " + cfg.GCP_SA_KEY_b64)
	}

	return cfg, nil
}

func hash(s string) uint32 {
	h := fnv.New32a()
	h.Write([]byte(s))
	return h.Sum32()
}
