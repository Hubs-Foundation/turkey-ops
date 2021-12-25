package handlers

import (
	"encoding/json"
	"errors"
	"fmt"
	"io/ioutil"
	"net/http"
	"regexp"
	"strconv"
	"strings"
	"time"

	"main/internal"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/service/cloudformation"
)

type clusterCfg struct {
	//required inputs
	Region                  string `json:"region`                   //us-east-1
	Domain                  string `json:"Domain"`                  //myhubs.net
	OAUTH_CLIENT_ID_FXA     string `json:"OAUTH_CLIENT_ID_FXA"`     //2db93e6523568888
	OAUTH_CLIENT_SECRET_FXA string `json:"OAUTH_CLIENT_SECRET_FXA"` //06e08133333333333387dd5425234388ac4e29999999999905a2eaea7e1d8888
	AWS_KEY                 string `json:"AWS_KEY"`                 //AKIAYEJRSWRAQSAM8888
	AWS_SECRET              string `json:"AWS_SECRET"`              //AKIAYEJRSWRAQSAM8888AKIAYEJRSWRAQSAM8888
	AWS_REGION              string `json:"AWS_REGION"`              //us-east-1
	//optional inputs
	Env             string `json:"env"`             //dev
	DeploymentName  string `json:"name"`            //z
	CF_deploymentId string `json:"cf_deploymentId"` //s0meid
	//produced here
	DB_PASS       string `json:"DB_PASS"`       //itjfHE8888
	COOKIE_SECRET string `json:"COOKIE_SECRET"` //a-random-string-to-sign-auth-cookies
	DB_HOST       string `json:"DB_HOST"`       //geng-test4turkey-db.ccgehrnbveo1.us-east-1.rds.amazonaws.com
	DB_CONN       string `json:"DB_CONN"`       //postgres://postgres:itjfHE8888@geng-test4turkey-db.ccgehrnbveo1.us-east-1.rds.amazonaws.com
	PERMS_KEY     string `json:"PERMS_KEY"`     //-----BEGIN RSA PRIVATE KEY-----\\nMIIEpgIBAAKCAQEA3RY0qLmdthY6Q0RZ4oyNQSL035BmYLNdleX1qVpG1zfQeLWf\\n/otgc8Ho2w8y5wW2W5vpI4a0aexNV2evgfsZKtx0q5WWwjsr2xy0Ak1zhWTgZD+F\\noHVGJ0xeFse2PnEhrtWalLacTza5RKEJskbNiTTu4fD+UfOCMctlwudNSs+AkmiP\\nSxc8nWrZ5BuvdnEXcJOuw0h4oyyUlkmj+Oa/ZQVH44lmPI9Ih0OakXWpIfOob3X0\\nXqcdywlMVI2hzBR3JNodRjyEz33p6E//lY4Iodw9NdcRpohGcxcgQ5vf4r4epLIa\\ncr0y5w1ZiRyf6BwyqJ6IBpA7yYpws3r9qxmAqwIDAQABAoIBAQCgwy/hbK9wo3MU\\nTNRrdzaTob6b/l1jfanUgRYEYl/WyYAu9ir0JhcptVwERmYGNVIoBRQfQClaSHjo\\n0L1/b74aO5oe1rR8Yhh+yL1gWz9gRT0hyEr7paswkkhsmiY7+3m5rxsrfinlM+6+\\nJ7dsSi3U0ofOBbZ4kvAeEz/Y3OaIOUbQraP312hQnTVQ3kp7HNi9GcLK9rq2mASu\\nO0DxDHXdZMsRN1K4tOKRZDsKGAEfL2jKN7+ndvsDhb4mAQaVKM8iw+g5O4HDA8uB\\nmwycaWhjilZWEyUyqvXE8tOMLS59sq6i1qrf8zIMWDOizebF/wnrQ42kzt5kQ0ZJ\\nwCPOC3sxAoGBAO6KfWr6WsXD6phnjVXXi+1j3azRKJGQorwQ6K3bXmISdlahngas\\nmBGBmI7jYTrPPeXAHUbARo/zLcbuGCf1sPipkAHYVC8f9aUbA205BREB15jNyXr3\\nXzhR/ronbn0VeR9iRua2FZjVChz22fdz9MvRJiinP8agYIQ4LovDk3lzAoGBAO1E\\nrZpOuv3TMQffPaPemWuvMYfZLgx2/AklgYqSoi683vid9HEEAdVzNWMRrOg0w5EH\\nWMEMPwJTYvy3xIgcFmezk5RMHTX2J32JzDJ8Y/uGf1wMrdkt3LkPRfuGepEDDtBa\\nrUSO/MeGXLu5p8QByUZkvTLJ4rJwF2HZBUehrm3pAoGBANg1+tveNCyRGbAuG/M0\\nvgXbwO+FXWojWP1xrhT3gyMNbOm079FI20Ty3F6XRmfRtF7stRyN5udPGaz33jlJ\\n/rBEsNybQiK8qyCNzZtQVYFG1C4SSI8GbO5Vk7cTSphhwDlsEKvJWuX+I36BWKts\\nFPQwjI/ImIvmjdUKP1Y7XQ51AoGBALWa5Y3ASRvStCqkUlfFH4TuuWiTcM2VnN+b\\nV4WrKnu/kKKWs+x09rpbzjcf5kptaGrvRp2sM+Yh0RhByCmt5fBF4OWXRJxy5lMO\\nT78supJgpcbc5YvfsJvs9tHIYrPvtT0AyrI5B33od74wIhrCiz5YCQCAygVuCleY\\ndpQXSp1RAoGBAKjasot7y/ErVxq7LIpGgoH+XTxjvMsj1JwlMeK0g3sjnun4g4oI\\nPBtpER9QaSFi2OeYPklJ2g2yvFcVzj/pFk/n1Zd9pWnbU+JIXBYaHTjmktLeZHsb\\nrTEKATo+Y1Alrhpr/z7gXXDfuKKXHkVRiper1YRAxELoLJB8r7LWeuIb\\n-----END RSA PRIVATE KEY-----
	PSQL          string `json:"PSQL"`          //postgresql://postgres:itjfHE8888@geng-test4turkey-db.ccgehrnbveo1.us-east-1.rds.amazonaws.com/ret_dev
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

		// #1. get cfg from r.body
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

		// #2. for keys in cfg start with 'cf_' prefix, pass into cloudformation without the prefix
		//     also generate password for values like "PwdGen(int_length)"
		cfParams, err := parseCFparams(cfg)
		if err != nil {
			sess.Panic("ERROR @ parseCFparams: " + err.Error())
		}
		// #3. add turkey cluster tags
		cfTags := []*cloudformation.Tag{
			{Key: aws.String("customer-id"), Value: aws.String("not-yet-place-holder-only")},
			{Key: aws.String("turkeyEnv"), Value: aws.String(cfg.Env)},
			{Key: aws.String("turkeyDomain"), Value: aws.String(cfg.Domain)},
		}

		// #4. run the cloudformation template
		stackName := cfg.DeploymentName + "-" + cfg.CF_deploymentId
		cfS3Folder := "https://s3.amazonaws.com/" + internal.Cfg.TurkeyCfg_s3_bkt + cfg.Env + "/cf/" + "/"
		go func() {
			err = awss.CreateCFstack(stackName, cfS3Folder+"main.yaml", cfParams, cfTags)
			if err != nil {
				sess.Panic("ERROR @ CreateCFstack for " + stackName + ": " + err.Error())
				return
			}
			// #4.1. post deployment configs
			// err = createSSMparam(stackName, cfg, awss)
			// if err != nil {
			// 	sess.Panic("post cf deployment: failed to createSSMparam: " + err.Error())
			// }
			k8sCfg, err := awss.GetK8sConfigFromEks(stackName)
			if err != nil {
				sess.Panic("post cf deployment: failed to get k8sCfg for eks name: " + stackName + "err: " + err.Error())
			}
			sess.Log("&#129311; k8s.k8sCfg.Host == " + k8sCfg.Host)

			yams, err := collectYams(cfg.Env, awss)
			if err != nil {
				sess.Panic("failed to collectYams: " + err.Error())
			}
			yamls, err := internal.K8s_render_yams(yams, cfg)
			if err != nil {
				sess.Panic("failed to K8s_render_yams: " + err.Error())
			}
			for _, yaml := range yamls {
				internal.Ssa_k8sChartYaml("turkey_cluster", yaml, k8sCfg)
			}
		}()
		sess.Log("&#128640;CreateCFstack started for stackName=" + stackName)

		go reportCreateCFstackStatus(stackName, cfg, sess, awss)

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
	if cfg.Domain == "" {
		return cfg, errors.New("bad input: Domain is required")
	}

	//optional inputs
	if cfg.Env == "" {
		cfg.Env = "dev"
		internal.GetLogger().Warn("Env unspecified -- using dev")
	}
	if cfg.DeploymentName == "" {
		cfg.DeploymentName = "z"
		internal.GetLogger().Warn("DeploymentName unspecified -- using z")
	}
	if cfg.CF_deploymentId == "" {
		cfg.CF_deploymentId = strconv.FormatInt(time.Now().Unix()-1626102245, 36)
		internal.GetLogger().Debug("CF_deploymentId unspecified -- using " + cfg.CF_deploymentId)
	}

	return cfg, nil
}

func parseCFparams(clusterCfg clusterCfg) ([]*cloudformation.Parameter, error) {

	var cfg map[string]string
	jCfg, err := json.Marshal(clusterCfg)
	if err != nil {
		return nil, err
	}
	json.Unmarshal(jCfg, &cfg)

	cfParams := []*cloudformation.Parameter{}

	for k := range cfg {
		if k[0:3] == "cf_" {
			key := k[3:]
			val := cfg[k]
			// isPwdGen, _ := regexp.MatchString(`PwdGen\(\d+\)`, val)
			if strings.HasPrefix(val, "PwdGen(") {
				compRegEx := regexp.MustCompile(`PwdGen\((?P<len>\d+)\)`)
				lenStr := compRegEx.FindStringSubmatch(val)[1]
				len, err := strconv.Atoi(lenStr)
				if err != nil {
					return nil, err
				}
				val = internal.PwdGen(len)
			}
			cfg[k] = val
			cfParams = append(cfParams,
				&cloudformation.Parameter{ParameterKey: aws.String(key), ParameterValue: aws.String(val)})
		}
	}
	return cfParams, nil
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
		env + "/k8s/cluster_00_deps.yam",
		env + "/k8s/cluster_01_ingress.yam",
		env + "/k8s/cluster_02_tools.yam",
		env + "/k8s/cluster_03_turkey-services.yam",
		env + "/k8s/cluster_04_turkey-stream.yam",
	} {
		yam, err := awss.S3Download_string(internal.Cfg.TurkeyCfg_s3_bkt, key)
		if err != nil {
			return yams, err
		}
		yams = append(yams, yam)
	}
	return yams, nil
}
