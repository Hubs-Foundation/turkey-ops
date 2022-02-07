package handlers

import (
	"context"
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"encoding/json"
	"encoding/pem"
	"errors"
	"io/ioutil"
	"net/http"
	"net/smtp"
	"os"
	"strconv"
	"strings"
	"time"

	"main/internal"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/service/autoscaling"
	"github.com/aws/aws-sdk-go/service/cloudformation"
	"github.com/aws/aws-sdk-go/service/ec2"
	"github.com/aws/aws-sdk-go/service/eks"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
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
	CF_DeploymentPrefix  string `json:"name"`                 //t-
	CF_deploymentId      string `json:"cf_deploymentId"`      //s0meid
	AWS_Ingress_Cert_ARN string `json:"aws_ingress_cert_arn"` //arn:aws:acm:us-east-1:123456605633:certificate/123456ab-f861-470b-a837-123456a76e17

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

var aws_yams = []string{
	"cluster_00_deps.yam",
	"cluster_01_ingress.yam",
	"cluster_02_tools.yam",
	"cluster_03_turkey-services.yam",
	"cluster_04_turkey-stream.yam",
	"cluster_autoscaler_aws.yam",
}

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
			sess.Error("ERROR @ turkey_makeCfg: " + err.Error())
			return
		}
		//aws service
		awss, err := internal.NewAwsSvs(cfg.AWS_KEY, cfg.AWS_SECRET, cfg.AWS_REGION)
		if err != nil {
			sess.Error("ERROR @ NewAwsSvs: " + err.Error())
			return
		}
		//test aws creds
		accountNum, err := awss.GetAccountID()
		if err != nil {
			sess.Error("ERROR @ GetAccountID: " + err.Error())
			return
		}
		sess.Log("good aws creds, account #: " + accountNum)
		// ################## 1.1 see if we have a *.<domain> cert in aws (acm) ###########
		cfg.AWS_Ingress_Cert_ARN, err = awss.ACM_findCertByDomainName("*."+cfg.Domain, "ISSUED")
		if err != nil {
			cfg.AWS_Ingress_Cert_ARN = `not-found_arn:aws:acm:<region>:<acct>:certificate/<id>`
		} else {
			sess.Log("found wildcard cert for <*." + cfg.Domain + "> in aws acm: " + cfg.AWS_Ingress_Cert_ARN)
		}
		// ################## 1.2 see if we have a <domain> in aws (route53) ##############
		err = awss.Route53_addRecord("*."+cfg.Domain, "CNAME", "deploying.myhubs.net")
		if err != nil {
			sess.Log(err.Error())
		} else {
			sess.Log("found " + cfg.Domain + " in route53")
		}

		// ######################################### 2. prepare params for cloudformation ##########################
		cfS3Folder := "https://s3.amazonaws.com/" + internal.Cfg.TurkeyCfg_s3_bkt + "/" + cfg.Env + "/cfs/"
		cfParams := parseCFparams(map[string]string{
			"deploymentId": cfg.CF_deploymentId,
			"cfS3Folder":   cfS3Folder,
			"turkeyDomain": cfg.Domain,
			"PGpwd":        cfg.DB_PASS,
		})
		cfTags := []*cloudformation.Tag{
			{Key: aws.String("customer-id"), Value: aws.String(r.Header.Get("X-Forwarded-UserEmail"))},
			{Key: aws.String("turkeyEnv"), Value: aws.String(cfg.Env)},
			{Key: aws.String("turkeyDomain"), Value: aws.String(cfg.Domain)},
		}
		// ######################################### 3. run cloudformation #########################################
		go func() {
			err = awss.CreateCFstack(cfg.CF_Stackname, cfS3Folder+"main.yaml", cfParams, cfTags)
			if err != nil {
				sess.Error("ERROR @ CreateCFstack for " + cfg.CF_Stackname + ": " + err.Error())
				return
			}

		}()
		sess.Log("&#128640;CreateCFstack started for stackName=" + cfg.CF_Stackname)
		reportCreateCFstackStatus(cfg.CF_Stackname, cfg, sess, awss)
		// ######################################### 4. post deployment configs ###################################
		report, err := postDeploymentConfigs(cfg, cfg.CF_Stackname, awss, r.Header.Get("X-Forwarded-UserEmail"), sess)
		if err != nil {
			sess.Error("ERROR @ postDeploymentConfigs for " + cfg.CF_Stackname + ": " + err.Error())
			return
		}
		// ################# 4.1. update CNAME if domain's managed in route53
		err = awss.Route53_addRecord("*."+cfg.Domain, "CNAME", report["lb"])
		if err != nil {
			sess.Log(err.Error())
		}
		sess.Log(" --- report for (" + cfg.CF_deploymentId + ") --- sknoonerToken: " + report["skoonerToken"])
		sess.Log(" --- report for (" + cfg.CF_deploymentId + ") --- you must create this CNAME record manually in your nameserver {" +
			cfg.Domain + ":" + report["lb"] + "}, and then go to dash." + cfg.Domain + " to view and configure cluster access")
		sess.Log("done: " + cfg.CF_deploymentId)

		return

	default:
		return
		// fmt.Fprintf(w, "unexpected method: "+r.Method)
	}
})

func postDeploymentConfigs(cfg clusterCfg, stackName string, awss *internal.AwsSvs, authnUser string, sess *internal.CacheBoxSessData) (map[string]string, error) {
	cfParams, err := getCfOutputParamMap(stackName, awss)
	if err != nil {
		sess.Error("post cf deployment: failed to getCfOutputParamMap: " + err.Error())
		return nil, err
	}
	cfg.DB_HOST = cfParams["DB_HOST"]
	k8sCfg, err := awss.GetK8sConfigFromEks(stackName)
	if err != nil {
		sess.Error("post cf deployment: failed to get k8sCfg for eks name: " + stackName + "err: " + err.Error())
		return nil, err
	}
	sess.Log("&#129311; k8s.k8sCfg.Host == " + k8sCfg.Host)
	cfg.DB_CONN = "postgres://postgres:" + cfg.DB_PASS + "@" + cfg.DB_HOST
	cfg.PSQL = "postgresql://postgres:" + cfg.DB_PASS + "@" + cfg.DB_HOST + "/ret_dev"

	// cfg.AWS_Ingress_Cert_ARN, err = awss.ACM_findCertByDomainName(cfg.Domain)
	// if err != nil {
	// 	sess.Error("ACM_findCertByDomainName err: " + err.Error())
	// 	cfg.AWS_Ingress_Cert_ARN = `fix-me_arn:aws:acm:<region>:<acct>:certificate/<id>`
	// }

	yams, err := collectYams(cfg.Env, awss)
	if err != nil {
		sess.Error("failed @ collectYams: " + err.Error())
		return nil, err
	}
	yamls, err := internal.K8s_render_yams(yams, cfg)
	if err != nil {
		sess.Error("post cf deployment: failed @ K8s_render_yams: " + err.Error())
		return nil, err
	}

	for _, yaml := range yamls {
		err := internal.Ssa_k8sChartYaml("turkey_cluster", yaml, k8sCfg)
		if err != nil {
			sess.Error("post cf deployment: failed @ Ssa_k8sChartYaml" + err.Error())
			return nil, err
		}
	}
	report := make(map[string]string)
	toolsSecrets, err := internal.K8s_GetAllSecrets(k8sCfg, "tools")
	if err != nil {
		sess.Error("post cf deployment: failed to get k8s secrets in tools namespace because: " + err.Error())
		return nil, err
	}
	for k, v := range toolsSecrets {
		if strings.HasPrefix(k, "skooner-sa-token-") {
			report["skoonerToken"] = string(v["token"])
		}
	}
	internal.GetLogger().Debug("~~~~~~~~~~skoonerToken: " + report["skoonerToken"])
	lb, err := internal.K8s_GetServiceHostName(k8sCfg, "ingress", "lb")
	if err != nil {
		sess.Error("post cf deployment: failed to get ingress lb's external ip because: " + err.Error())
		return nil, err
	}
	report["lb"] = lb
	internal.GetLogger().Debug("~~~~~~~~~~lb: " + report["lb"])

	//----------------------------------------
	err = eksIpLimitationFix(k8sCfg, awss, stackName)
	if err != nil {
		sess.Error("@eksIpLimitationFix: " + err.Error())
	}
	//

	//email the final manual steps to authenticated user
	clusterCfgBytes, _ := json.Marshal(cfg)
	err = smtp.SendMail(
		internal.Cfg.SmtpServer+":"+internal.Cfg.SmtpPort,
		smtp.PlainAuth("", internal.Cfg.SmtpUser, internal.Cfg.SmtpPass, internal.Cfg.SmtpServer),
		"noreply@"+internal.Cfg.Domain,
		[]string{authnUser, "gtan@mozilla.com"},
		[]byte("To: "+authnUser+"\r\n"+
			"Subject: turkey_aws deployment <"+stackName+"> \r\n"+
			"\r\n******required manual steps******"+
			"\r\n- CNAME required: *."+cfg.Domain+" : "+report["lb"]+
			"\r\n******optional manual steps******"+
			"\r\n- update https cert at: https://dash."+cfg.Domain+"/#!service/ingress/lb"+
			"\r\n- for aws-eks, update role mappings at: https://dash."+cfg.Domain+"/#!configmap/kube-system/aws-auth"+
			"\r\n******things you need******"+
			"\r\n- sknoonerToken: "+report["skoonerToken"]+
			"\r\n******clusterCfg******"+
			"\r\n"+string(clusterCfgBytes)+
			"\r\n"),
	)
	if err != nil {
		internal.GetLogger().Error(err.Error())
	}

	return report, nil
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

func reportCreateCFstackStatus(stackName string, cfg clusterCfg, sess *internal.CacheBoxSessData, awss *internal.AwsSvs) error {
	time.Sleep(time.Second * 10)
	stackStatus := "something something IN_PROGRESS"
	tries := 3
	for strings.Contains(stackStatus, "IN_PROGRESS") {
		if tries < 1 {
			sess.Error("failed @ reportCreateCFstackStatus: timeout")
			return errors.New("timeout")
		}
		stacks, err := awss.GetStack(stackName)
		if err != nil {
			tries--
			sess.Error("@ reportCreateCFstackStatus -- err: " + err.Error())
			sess.Error("@ reportCreateCFstackStatus -- tries left" + strconv.Itoa(tries))
			time.Sleep(time.Second * 30)
			continue
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
	for _, s3Key := range aws_yams {
		yanS3 := env + "/yams/" + s3Key
		yam, err := awss.S3Download_string(internal.Cfg.TurkeyCfg_s3_bkt, yanS3)
		if err != nil {
			return yams, err
		}
		yams = append(yams, yam)
	}
	return yams, nil
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

////////////////////////////////// eks pod ip config hack ///////////////////////
// ref: https://docs.aws.amazon.com/eks/latest/userguide/cni-increase-ip-addresses.html
// because:
//	1. (as of 2/7/2022) this statement from above doc is not true -- "When you deploy a 1.21 or later cluster, version 1.10.1 or later of the VPC CNI add-on is deployed with it, and this setting is true by default."
//	2. (as of 2/7/2022) eks is BAD (also from above doc): "If you have an existing managed node group, the next AMI or launch template update of your node group results in new worker nodes coming up with the new IP address prefix assignment-enabled max-pod value."
func eksIpLimitationFix(k8sCfg *rest.Config, as *internal.AwsSvs, stackName string) error {
	// 1. set ENABLE_PREFIX_DELEGATION to "true"
	clientSet, err := kubernetes.NewForConfig(k8sCfg)
	if err != nil {
		return err
	}
	ds, err := clientSet.AppsV1().DaemonSets("kube-system").Get(context.Background(), "aws-node", v1.GetOptions{})
	if err != nil {
		return err
	}
	found := false
	for i := range ds.Spec.Template.Spec.Containers[0].Env {
		if ds.Spec.Template.Spec.Containers[0].Env[i].Name == "ENABLE_PREFIX_DELEGATION" {
			ds.Spec.Template.Spec.Containers[0].Env[i].Value = "true"
			found = true
			break
		}
	}
	if !found {
		return errors.New("did not find envVar <ENABLE_PREFIX_DELEGATION>")
	}
	clientSet.AppsV1().DaemonSets("kube-system").Update(context.Background(), ds, v1.UpdateOptions{})
	// 2. make a (fake) new revision for the nodegroup's launchTemplate
	eksClient := eks.New(as.Sess)
	ng, err := eksClient.DescribeNodegroup(&eks.DescribeNodegroupInput{
		ClusterName:   &stackName,
		NodegroupName: aws.String(stackName + "-ng"),
	})
	if err != nil {
		return err
	}
	ec2Client := ec2.New(as.Sess)
	_, err = ec2Client.CreateLaunchTemplateVersion(&ec2.CreateLaunchTemplateVersionInput{
		LaunchTemplateId:   ng.Nodegroup.LaunchTemplate.Id,
		LaunchTemplateData: &ec2.RequestLaunchTemplateData{},
		SourceVersion:      aws.String("1"),
		VersionDescription: aws.String("0"),
	})
	if err != nil {
		return err
	}
	// 3. update the nodegroup with new launchTemplate
	// ng.SetNodegroup(&eks.Nodegroup{
	// 	LaunchTemplate: &eks.LaunchTemplateSpecification{
	// 		Id:      ng.Nodegroup.LaunchTemplate.Id,
	// 		Version: aws.String("2"),
	// 	},
	// })
	asg := autoscaling.New(as.Sess)
	asg.UpdateAutoScalingGroup(&autoscaling.UpdateAutoScalingGroupInput{
		AutoScalingGroupName: ng.Nodegroup.Resources.AutoScalingGroups[0].Name,
		LaunchTemplate: &autoscaling.LaunchTemplateSpecification{
			LaunchTemplateId: ng.Nodegroup.LaunchTemplate.Id,
			Version:          aws.String("2"),
		},
	})

	return nil
}

//////////////////////////////////////////////////////////////////////////////////
