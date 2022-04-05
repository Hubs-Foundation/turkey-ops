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
	"time"
)

var logger = internal.GetLogger()

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
	// this will just be Region ...
	AWS_REGION string `json:"AWS_REGION"` //us-east-1
	ItaChan    string `json:"itachan"`    //ita's listening channel (dev, beta, stable), falls back to Env / swaping staging for beta

	//optional inputs
	DeploymentPrefix     string `json:"name"`                 //t-
	DeploymentId         string `json:"deploymentId"`         //s0meid
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
		logger.Warn("ERROR @ reading r.body, error = " + err.Error())
		return cfg, err
	}
	//make cfg
	err = json.Unmarshal(rBodyBytes, &cfg)
	if err != nil {
		logger.Warn("bad clusterCfg: " + string(rBodyBytes))
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
		logger.Warn("OAUTH_CLIENT_ID_FXA not supplied, falling back to local value")
		cfg.OAUTH_CLIENT_ID_FXA = os.Getenv("OAUTH_CLIENT_ID_FXA")
	}
	if cfg.OAUTH_CLIENT_SECRET_FXA == "" {
		logger.Warn("OAUTH_CLIENT_SECRET_FXA not supplied, falling back to local value")
		cfg.OAUTH_CLIENT_SECRET_FXA = os.Getenv("OAUTH_CLIENT_SECRET_FXA")
	}
	if cfg.SMTP_SERVER == "" {
		logger.Warn("SMTP_SERVER not supplied, falling back to local value")
		cfg.SMTP_SERVER = internal.Cfg.SmtpServer
	}
	if cfg.SMTP_PORT == "" {
		logger.Warn("SMTP_PORT not supplied, falling back to local value")
		cfg.SMTP_PORT = internal.Cfg.SmtpPort
	}
	if cfg.SMTP_USER == "" {
		logger.Warn("SMTP_USER not supplied, falling back to local value")
		cfg.SMTP_USER = internal.Cfg.SmtpUser
	}
	if cfg.SMTP_PASS == "" {
		logger.Warn("SMTP_PASS not supplied, falling back to local value")
		cfg.SMTP_PASS = internal.Cfg.SmtpPass
	}
	if cfg.AWS_KEY == "" {
		logger.Warn("AWS_KEY not supplied, falling back to local value")
		cfg.AWS_KEY = internal.Cfg.AwsKey
	}
	if cfg.AWS_SECRET == "" {
		logger.Warn("AWS_SECRET not supplied, falling back to local value")
		cfg.AWS_SECRET = internal.Cfg.AwsSecret
	}
	if cfg.Env == "" {
		cfg.Env = "dev"
		logger.Warn("Env unspecified -- using dev")
	}
	if cfg.ItaChan == "" {
		cfg.ItaChan = strings.Replace(cfg.Env, "staging", "beta", 1)
		logger.Warn("ItaChan unspecified -- fallinb back to Env (swaping staging for beta): " + cfg.ItaChan)
	}

	//optional inputs
	if cfg.DeploymentPrefix == "" {
		cfg.DeploymentPrefix = "t-"
		logger.Warn("deploymentPrefix unspecified -- using (default)" + cfg.DeploymentPrefix)
	}
	if cfg.DeploymentId == "" {
		cfg.DeploymentId = strconv.FormatInt(time.Now().Unix()-1648672222, 36)
		logger.Info("deploymentId: " + cfg.DeploymentId)
	}
	if cfg.Stackname == "" {
		cfg.Stackname = cfg.DeploymentPrefix + cfg.DeploymentId
	}

	//generate the rest
	pwdSeed := int64(hash(cfg.Stackname))
	cfg.DB_PASS = internal.PwdGen(15, pwdSeed, "D~")
	cfg.COOKIE_SECRET = internal.PwdGen(15, pwdSeed, "C~")
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
		logger.Warn("GCP_SA_KEY_b64 unspecified -- using: " + cfg.GCP_SA_KEY_b64)
	}

	return cfg, nil
}

func hash(s string) uint32 {
	h := fnv.New32a()
	h.Write([]byte(s))
	return h.Sum32()
}
