package handlers

import (
	"encoding/json"
	"net/http"
	"regexp"
	"strconv"
	"strings"
	"time"

	"main/internal"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/service/cloudformation"
)

// type clusterCfg struct {
// 	Env             string `json:"env"`
// 	DeploymentName  string `json:"name"`
// 	CF_deploymentId string `json:"cf_deploymentId"`
// 	Domain          string `json:"domain"`
// }

var turkeyEnv = "dev"
var turkeycfg_s3_bucket = "turkeycfg/cf/"
var region = "us-west-1"

var TcoAws = http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case "POST":
		if r.URL.Path != "/tco_aws" {
			http.Error(w, http.StatusText(http.StatusNotFound), http.StatusNotFound)
			return
		}

		sess := internal.GetSession(r.Cookie)
		sess.Log("!!! THE ONE BUTTON clicked !!!")

		cfg, err := internal.ParseJsonReqBody(r.Body)
		if err != nil {
			sess.Log("ERROR @ Unmarshal r.body, will try configs in cache, btw json.Unmarshal error = " + err.Error())
			return
		}
		if cfg["Region"] == "" {
			internal.GetLogger().Warn("no region input, using default: " + region)
		}

		awss, err := internal.NewAwsSvs(internal.Cfg.AwsKey, internal.Cfg.AwsSecret, region)
		if err != nil {
			sess.Log("ERROR @ NewAwsSvs: " + err.Error())
			return
		}

		accountNum, err := awss.GetAccountID()
		if err != nil {
			sess.Log("ERROR @ GetAccountID: " + err.Error())
			return
		}
		sess.Log("good aws creds, account #: " + accountNum)

		turkeyDomain, gotDomain := cfg["Domain"]
		if !gotDomain {
			internal.GetLogger().Panic("missing: Domain")
		}

		deploymentName, ok := cfg["DeploymentName"]
		if !ok {
			deploymentName = "z"
		}

		turkeyEnv, ok = cfg["Env"]
		if !ok {
			turkeyEnv = "dev"
		}
		stackName := deploymentName + "-" + internal.StackNameGen()

		_, ok = cfg["CF_deploymentId"]
		if !ok {
			cfg["CF_deploymentId"] = strconv.FormatInt(time.Now().Unix()-1626102245, 36)
		}

		cfS3Folder := "https://s3.amazonaws.com/" + turkeycfg_s3_bucket + turkeyEnv + "/"
		cfg["cf_cfS3Folder"] = cfS3Folder

		cfParams, err := parseCFparams(cfg)
		if err != nil {
			sess.Log("ERROR @ parseCFparams: " + err.Error())
		}
		cfTags := []*cloudformation.Tag{
			{Key: aws.String("customer-id"), Value: aws.String("not-yet-place-holder-only")},
			{Key: aws.String("turkeyEnv"), Value: aws.String(turkeyEnv)},
			{Key: aws.String("turkeyDomain"), Value: aws.String(turkeyDomain)},
		}

		go func() {
			err = awss.CreateCFstack(stackName, cfS3Folder+"main.yaml", cfParams, cfTags)
			if err != nil {
				sess.Log("ERROR @ CreateCFstack for " + stackName + ": " + err.Error())
				return
			}
			// createSSMparam(stackName, accountNum, cfg, awss, sess)
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
		// 	stackName+"-assets-"+cfg["CF_deploymentId"])

		// go internal.DeployKeys(awss, stackName, stackName+"-assets-"+cfg["CF_deploymentId"])

		return

	default:
		return
		// fmt.Fprintf(w, "unexpected method: "+r.Method)
	}
})

func parseCFparams(cfg map[string]string) ([]*cloudformation.Parameter, error) {

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

func createSSMparam(stackName string, accountNum string, cfg map[string]string, awss *internal.AwsSvs, sess *internal.CacheBoxSessData) error {
	stacks, err := awss.GetStack(stackName)
	if err != nil {
		sess.Log("ERROR @ createSSMparam -- GetStack: " + err.Error())
		return err
	}
	//----------create SSM parameter
	// paramMap, err := getSSMparamFromS3json(awss, cfg, "ssmParam.json")
	// if err != nil {
	// 	sess.Log("ERROR @ createSSMparamFromS3json: " + err.Error())
	// 	return err
	// }
	paramMap := make(map[string]string)
	for _, k := range cfg {
		if k[0:3] == "cf_" {
			paramMap[k[3:]] = cfg[k]
		}
	}
	stackOutputs := stacks[0].Outputs
	for _, output := range stackOutputs {
		paramMap[*output.Description] = *output.OutputValue
	}

	paramJSONbytes, _ := json.Marshal(paramMap)
	err = awss.CreateSSMparameter(stackName, string(paramJSONbytes))
	if err != nil {
		sess.Log("ERROR @ createSSMparamFromS3json: " + err.Error())
		return err
	}
	return nil
}

func reportCreateCFstackStatus(stackName string, cfg map[string]string, sess *internal.CacheBoxSessData, awss *internal.AwsSvs) error {
	time.Sleep(time.Second * 10)
	stackStatus := "something something IN_PROGRESS"
	for strings.Contains(stackStatus, "IN_PROGRESS") {
		stacks, err := awss.GetStack(stackName)
		if err != nil {
			sess.Log("ERROR @ reportCreateCFstackStatus: " + err.Error())
			return err
		}
		stack := *stacks[0]
		stackStatus = *stack.StackStatus
		sinceStart := time.Now().UTC().Sub(stack.CreationTime.UTC()).Round(time.Second).String()
		stackLink := "https://" + region + ".console.aws.amazon.com/cloudformation/home?region=" + region + "#/stacks/stackinfo?stackId=" + *stack.StackId
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
