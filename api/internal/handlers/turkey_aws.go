package handlers

import (
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"encoding/json"
	"encoding/pem"
	"errors"
	"fmt"
	"io/ioutil"
	"net/http"
	"os"
	"strconv"
	"strings"
	"time"

	"main/internal"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/service/cloudformation"
)

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
	DeploymentName  string `json:"name"`            //z
	CF_deploymentId string `json:"cf_deploymentId"` //s0meid

	//generated pre-infra-deploy
	DB_PASS       string `json:"DB_PASS"`       //itjfHE8888
	COOKIE_SECRET string `json:"COOKIE_SECRET"` //a-random-string-to-sign-auth-cookies
	PERMS_KEY     string `json:"PERMS_KEY"`     //-----BEGIN RSA PRIVATE KEY-----\\nMIIEpgIBAAKCAQEA3RY0qLmdthY6Q0RZ4oyNQSL035BmYLNdleX1qVpG1zfQeLWf\\n/otgc8Ho2w8y5wW2W5vpI4a0aexNV2evgfsZKtx0q5WWwjsr2xy0Ak1zhWTgZD+F\\noHVGJ0xeFse2PnEhrtWalLacTza5RKEJskbNiTTu4fD+UfOCMctlwudNSs+AkmiP\\nSxc8nWrZ5BuvdnEXcJOuw0h4oyyUlkmj+Oa/ZQVH44lmPI9Ih0OakXWpIfOob3X0\\nXqcdywlMVI2hzBR3JNodRjyEz33p6E//lY4Iodw9NdcRpohGcxcgQ5vf4r4epLIa\\ncr0y5w1ZiRyf6BwyqJ6IBpA7yYpws3r9qxmAqwIDAQABAoIBAQCgwy/hbK9wo3MU\\nTNRrdzaTob6b/l1jfanUgRYEYl/WyYAu9ir0JhcptVwERmYGNVIoBRQfQClaSHjo\\n0L1/b74aO5oe1rR8Yhh+yL1gWz9gRT0hyEr7paswkkhsmiY7+3m5rxsrfinlM+6+\\nJ7dsSi3U0ofOBbZ4kvAeEz/Y3OaIOUbQraP312hQnTVQ3kp7HNi9GcLK9rq2mASu\\nO0DxDHXdZMsRN1K4tOKRZDsKGAEfL2jKN7+ndvsDhb4mAQaVKM8iw+g5O4HDA8uB\\nmwycaWhjilZWEyUyqvXE8tOMLS59sq6i1qrf8zIMWDOizebF/wnrQ42kzt5kQ0ZJ\\nwCPOC3sxAoGBAO6KfWr6WsXD6phnjVXXi+1j3azRKJGQorwQ6K3bXmISdlahngas\\nmBGBmI7jYTrPPeXAHUbARo/zLcbuGCf1sPipkAHYVC8f9aUbA205BREB15jNyXr3\\nXzhR/ronbn0VeR9iRua2FZjVChz22fdz9MvRJiinP8agYIQ4LovDk3lzAoGBAO1E\\nrZpOuv3TMQffPaPemWuvMYfZLgx2/AklgYqSoi683vid9HEEAdVzNWMRrOg0w5EH\\nWMEMPwJTYvy3xIgcFmezk5RMHTX2J32JzDJ8Y/uGf1wMrdkt3LkPRfuGepEDDtBa\\nrUSO/MeGXLu5p8QByUZkvTLJ4rJwF2HZBUehrm3pAoGBANg1+tveNCyRGbAuG/M0\\nvgXbwO+FXWojWP1xrhT3gyMNbOm079FI20Ty3F6XRmfRtF7stRyN5udPGaz33jlJ\\n/rBEsNybQiK8qyCNzZtQVYFG1C4SSI8GbO5Vk7cTSphhwDlsEKvJWuX+I36BWKts\\nFPQwjI/ImIvmjdUKP1Y7XQ51AoGBALWa5Y3ASRvStCqkUlfFH4TuuWiTcM2VnN+b\\nV4WrKnu/kKKWs+x09rpbzjcf5kptaGrvRp2sM+Yh0RhByCmt5fBF4OWXRJxy5lMO\\nT78supJgpcbc5YvfsJvs9tHIYrPvtT0AyrI5B33od74wIhrCiz5YCQCAygVuCleY\\ndpQXSp1RAoGBAKjasot7y/ErVxq7LIpGgoH+XTxjvMsj1JwlMeK0g3sjnun4g4oI\\nPBtpER9QaSFi2OeYPklJ2g2yvFcVzj/pFk/n1Zd9pWnbU+JIXBYaHTjmktLeZHsb\\nrTEKATo+Y1Alrhpr/z7gXXDfuKKXHkVRiper1YRAxELoLJB8r7LWeuIb\\n-----END RSA PRIVATE KEY-----
	//generated post-infra-deploy
	DB_HOST string `json:"DB_HOST"` //geng-test4turkey-db.ccgehrnbveo1.us-east-1.rds.amazonaws.com
	DB_CONN string `json:"DB_CONN"` //postgres://postgres:itjfHE8888@geng-test4turkey-db.ccgehrnbveo1.us-east-1.rds.amazonaws.com
	PSQL    string `json:"PSQL"`    //postgresql://postgres:itjfHE8888@geng-test4turkey-db.ccgehrnbveo1.us-east-1.rds.amazonaws.com/ret_dev
}

// var turkeycfg_s3_bucket = "turkeycfg/cf/"
// var defaultRegion = "us-east-1"

var TurkeyAws = http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case "POST":
		if r.URL.Path != "/tco_aws" {
			http.Error(w, http.StatusText(http.StatusNotFound), http.StatusNotFound)
			return
		}
		sess := internal.GetSession(r.Cookie)

		// ######################################### 1. get cfg from r.body ########################################
		cfg, err := turkey_makeCfg(r, sess)
		if err != nil {
			sess.Panic("ERROR @ turkey_makeCfg: " + err.Error())
		}

		awss, err := internal.NewAwsSvs(internal.Cfg.AwsKey, internal.Cfg.AwsSecret, cfg.Region)
		if err != nil {
			sess.Panic("ERROR @ NewAwsSvs: " + err.Error())
			return
		}

		accountNum, err := awss.GetAccountID()
		if err != nil {
			sess.Panic("ERROR @ GetAccountID: " + err.Error())
			return
		}
		sess.Log("good aws creds, account #: " + accountNum)

		// ######################################### 2. prepare params for cloudformation ##########################
		cfS3Folder := "https://s3.amazonaws.com/" + internal.Cfg.TurkeyCfg_s3_bkt + "/" + cfg.Env + "/cfs/"
		cfParams := parseCFparams(map[string]string{
			"deploymentId": cfg.CF_deploymentId,
			"cfS3Folder":   cfS3Folder,
			"turkeyDomain": cfg.Domain,
			"PGpwd":        cfg.DB_PASS,
		})
		cfTags := []*cloudformation.Tag{
			{Key: aws.String("customer-id"), Value: aws.String("not-yet-place-holder-only")},
			{Key: aws.String("turkeyEnv"), Value: aws.String(cfg.Env)},
			{Key: aws.String("turkeyDomain"), Value: aws.String(cfg.Domain)},
		}
		// ######################################### 3. run cloudformation #########################################
		stackName := cfg.DeploymentName + "-" + cfg.CF_deploymentId
		go func() {
			err = awss.CreateCFstack(stackName, cfS3Folder+"main.yaml", cfParams, cfTags)
			if err != nil {
				sess.Panic("ERROR @ CreateCFstack for " + stackName + ": " + err.Error())
				return
			}

		}()
		sess.Log("&#128640;CreateCFstack started for stackName=" + stackName)

		// go reportCreateCFstackStatus(stackName, cfg, sess, awss)
		reportCreateCFstackStatus(stackName, cfg, sess, awss)

		// ######################################### 4. post deployment configs ###################################
		report, err := postDeploymentConfigs(cfg, stackName, awss, sess)
		if err != nil {
			sess.Panic("ERROR @ postDeploymentConfigs for " + stackName + ": " + err.Error())
		}
		sess.Log(" --- report for (" + cfg.CF_deploymentId + ") --- sknoonerToken: " + report["skoonerToken"])
		sess.Log(" --- report for (" + cfg.CF_deploymentId + ") --- you must create this CNAME record manually in your nameserver {" +
			cfg.Domain + ":" + report["lb"] + "}, and then go to dash." + cfg.Domain + " to view and configure cluster access")
		sess.Log("done: " + cfg.CF_deploymentId)
		// go internal.DeployHubsAssets(
		// 	awss,
		// 	map[string]string{
		// 		"base_assets_path":       "https://" + stackName + "-cdn." + turkeyDomain + "/hubs/",
		// 		"cors_proxy_server":      "",
		// 		"ga_tracking_id":         "",
		// 		"ita_server":             "",
		// 		"non_cors_proxy_domains": stackName + "." + turkeyDomain + "," + stackName + "-cdn." + turkeyDomain,
		// 		"postgrest_server":       "",
		// 		"reticulum_server":       stackName + "." + turkeyDomain,
		// 		"sentry_dsn":             "",
		// 		"shortlink_domain":       "notyet.link",
		// 		"thumbnail_server":       "notyet.com",
		// 	},
		// 	turkeycfg_s3_bucket,
		// 	stackName+"-assets-"+cfg.CF_deploymentId)

		// go internal.DeployKeys(awss, stackName, stackName+"-assets-"+cfg.CF_deploymentId)

		return

	default:
		return
		// fmt.Fprintf(w, "unexpected method: "+r.Method)
	}
})

func postDeploymentConfigs(cfg clusterCfg, stackName string, awss *internal.AwsSvs, sess *internal.CacheBoxSessData) (map[string]string, error) {
	cfParams, err := getCfOutputParamMap(stackName, awss)
	if err != nil {
		sess.Panic("post cf deployment: failed to getCfOutputParamMap: " + err.Error())
	}
	cfg.DB_HOST = cfParams["DB_HOST"]
	k8sCfg, err := awss.GetK8sConfigFromEks(stackName)
	if err != nil {
		sess.Panic("post cf deployment: failed to get k8sCfg for eks name: " + stackName + "err: " + err.Error())
	}
	sess.Log("&#129311; k8s.k8sCfg.Host == " + k8sCfg.Host)
	cfg.DB_CONN = "postgres://postgres:" + cfg.DB_PASS + "@geng-test4turkey-db.ccgehrnbveo1.us-east-1.rds.amazonaws.com"

	yams, err := collectYams(cfg.Env, awss)
	if err != nil {
		sess.Panic("failed @ collectYams: " + err.Error())
	}
	yamls, err := internal.K8s_render_yams(yams, cfg)
	if err != nil {
		sess.Panic("post cf deployment: failed @ K8s_render_yams: " + err.Error())
	}

	for _, yaml := range yamls {
		err := internal.Ssa_k8sChartYaml("turkey_cluster", yaml, k8sCfg)
		if err != nil {
			sess.Panic("post cf deployment: failed @ Ssa_k8sChartYaml" + err.Error())
		}
	}
	report := make(map[string]string)
	toolsSecrets, err := internal.K8s_GetAllSecrets(k8sCfg, "tools")
	if err != nil {
		sess.Panic("post cf deployment: failed to get k8s secrets in tools namespace because: " + err.Error())
	}
	for k, v := range toolsSecrets {
		if strings.HasPrefix(k, "skooner-sa-token-") {
			report["skoonerToken"] = string(v["token"])
		}
	}
	fmt.Println("~~~~~~~~~~skoonerToken: " + report["skoonerToken"])
	lb, err := internal.K8s_GetServiceHostName(k8sCfg, "ingress", "lb")
	if err != nil {
		sess.Panic("post cf deployment: failed to get ingress lb's external ip because: " + err.Error())
	}
	report["lb"] = lb
	fmt.Println("~~~~~~~~~~lb: " + report["lb"])

	return report, nil
}

func turkey_makeCfg(r *http.Request, sess *internal.CacheBoxSessData) (clusterCfg, error) {
	var cfg clusterCfg

	//get r.body
	rBodyBytes, err := ioutil.ReadAll(r.Body)
	if err != nil {
		fmt.Println("ERROR @ reading r.body, error = " + err.Error())
		return cfg, err
	}
	//make cfg
	err = json.Unmarshal(rBodyBytes, &cfg)
	if err != nil {
		fmt.Println("bad clusterCfg: " + string(rBodyBytes))
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
		fallback := os.Getenv("OAUTH_CLIENT_ID_FXA")
		internal.GetLogger().Warn("OAUTH_CLIENT_ID_FXA not supplied, falling back to: " + fallback)
		cfg.OAUTH_CLIENT_ID_FXA = fallback
	}
	if cfg.OAUTH_CLIENT_SECRET_FXA == "" {
		fallback := os.Getenv("OAUTH_CLIENT_SECRET_FXA")
		internal.GetLogger().Warn("OAUTH_CLIENT_SECRET_FXA not supplied, falling back to: " + fallback)
		cfg.OAUTH_CLIENT_SECRET_FXA = fallback
	}
	if cfg.SMTP_SERVER == "" {
		fallback := internal.Cfg.SmtpServer
		internal.GetLogger().Warn("SMTP_SERVER not supplied, falling back to: " + fallback)
		cfg.SMTP_SERVER = fallback
	}
	if cfg.SMTP_PORT == "" {
		fallback := internal.Cfg.SmtpPort
		internal.GetLogger().Warn("SMTP_PORT not supplied, falling back to: " + fallback)
		cfg.SMTP_PORT = fallback
	}
	if cfg.SMTP_USER == "" {
		fallback := internal.Cfg.SmtpUser
		internal.GetLogger().Warn("SMTP_USER not supplied, falling back to: " + fallback)
		cfg.SMTP_USER = fallback
	}
	if cfg.SMTP_PASS == "" {
		fallback := internal.Cfg.SmtpPass
		internal.GetLogger().Warn("SMTP_PASS not supplied, falling back to: " + fallback)
		cfg.SMTP_PASS = fallback
	}
	if cfg.AWS_KEY == "" {
		fallback := internal.Cfg.AwsKey
		internal.GetLogger().Warn("AWS_KEY not supplied, falling back to: " + fallback)
		cfg.AWS_KEY = fallback
	}
	if cfg.AWS_SECRET == "" {
		fallback := internal.Cfg.AwsSecret
		internal.GetLogger().Warn("AWS_SECRET not supplied, falling back to: " + fallback)
		cfg.AWS_SECRET = fallback
	}
	if cfg.Env == "" {
		cfg.Env = "dev"
		internal.GetLogger().Warn("Env unspecified -- using dev")
	}

	//optional inputs
	if cfg.DeploymentName == "" {
		cfg.DeploymentName = "z"
		internal.GetLogger().Warn("DeploymentName unspecified -- using z")
	}
	if cfg.CF_deploymentId == "" {
		cfg.CF_deploymentId = strconv.FormatInt(time.Now().Unix()-1626102245, 36)
		internal.GetLogger().Info("CF_deploymentId: " + cfg.CF_deploymentId)
	}

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

func parseCFparams(cfg map[string]string) []*cloudformation.Parameter {
	cfParams := []*cloudformation.Parameter{}
	for key, val := range cfg {
		cfParams = append(cfParams,
			&cloudformation.Parameter{ParameterKey: aws.String(key), ParameterValue: aws.String(val)})
	}
	return cfParams
}

func getCfOutputParamMap(stackName string, awss *internal.AwsSvs) (map[string]string, error) {
	paramMap := make(map[string]string)
	stacks, err := awss.GetStack(stackName)
	if err != nil {
		return paramMap, err
	}
	for _, output := range stacks[0].Outputs {
		paramMap[*output.Description] = *output.OutputValue
	}
	return paramMap, nil
}

// func createSSMparam(stackName string, cfg map[string]string, awss *internal.AwsSvs) error {
// 	stacks, err := awss.GetStack(stackName)
// 	if err != nil {
// 		return err
// 	}
// 	//----------create SSM parameter
// 	// paramMap, err := getSSMparamFromS3json(awss, cfg, "ssmParam.json")
// 	// if err != nil {
// 	// 	sess.Log("ERROR @ createSSMparamFromS3json: " + err.Error())
// 	// 	return err
// 	// }
// 	paramMap := make(map[string]string)
// 	for _, k := range cfg {
// 		if k[0:3] == "cf_" {
// 			paramMap[k[3:]] = cfg[k]
// 		}
// 	}
// 	stackOutputs := stacks[0].Outputs
// 	for _, output := range stackOutputs {
// 		paramMap[*output.Description] = *output.OutputValue
// 	}

// 	paramJSONbytes, _ := json.Marshal(paramMap)
// 	err = awss.CreateSSMparameter(stackName, string(paramJSONbytes))
// 	if err != nil {
// 		return err
// 	}
// 	return nil
// }

func reportCreateCFstackStatus(stackName string, cfg clusterCfg, sess *internal.CacheBoxSessData, awss *internal.AwsSvs) error {
	time.Sleep(time.Second * 10)
	stackStatus := "something something IN_PROGRESS"
	for strings.Contains(stackStatus, "IN_PROGRESS") {
		stacks, err := awss.GetStack(stackName)
		if err != nil {
			sess.Panic("ERROR @ reportCreateCFstackStatus: " + err.Error())
			return err
		}
		stack := *stacks[0]
		stackStatus = *stack.StackStatus
		sinceStart := time.Now().UTC().Sub(stack.CreationTime.UTC()).Round(time.Second).String()
		stackLink := "https://" + cfg.Region + ".console.aws.amazon.com/cloudformation/home?region=" + cfg.Region + "#/stacks/stackinfo?stackId=" + *stack.StackId
		reportMsg := "<span style=\"color:white\">(" + sinceStart + ")</span> status of CF stack " +
			"<a href=\"" + stackLink + "\" target=\"_blank\"><b>&#128279;" + stackName + "</b></a>" + " is " + stackStatus
		if stack.StackStatusReason != nil {
			reportMsg = reportMsg + " because " + *stack.StackStatusReason
		}
		sess.Log(reportMsg)
		time.Sleep(time.Second * 60)
	}
	return nil
}

func collectYams(env string, awss *internal.AwsSvs) ([]string, error) {
	var yams []string
	for _, key := range []string{
		env + "/yams/cluster_00_deps.yam",
		env + "/yams/cluster_01_ingress.yam",
		env + "/yams/cluster_02_tools.yam",
		env + "/yams/cluster_03_turkey-services.yam",
		env + "/yams/cluster_04_turkey-stream.yam",
	} {
		yam, err := awss.S3Download_string(internal.Cfg.TurkeyCfg_s3_bkt, key)
		if err != nil {
			return yams, err
		}
		yams = append(yams, yam)
	}
	return yams, nil
}
