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

// var turkeycfg_s3_bucket = "turkeycfg/cf/"
// var defaultRegion = "us-east-1"

var TcoAws = http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case "POST":
		if r.URL.Path != "/tco_aws" {
			http.Error(w, http.StatusText(http.StatusNotFound), http.StatusNotFound)
			return
		}
		sess := internal.GetSession(r.Cookie)

		// #1. get cfg from r.body
		cfg := tco_makeCfg(r, sess)

		awss, err := internal.NewAwsSvs(internal.Cfg.AwsKey, internal.Cfg.AwsSecret, cfg["Region"])
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
			{Key: aws.String("turkeyEnv"), Value: aws.String(cfg["Env"])},
			{Key: aws.String("turkeyDomain"), Value: aws.String(cfg["Domain"])},
		}

		// #4. run the cloudformation template
		stackName := cfg["DeploymentName"] + "-" + cfg["cf_deploymentId"]
		cfS3Folder := "https://s3.amazonaws.com/" + internal.Cfg.TurkeyCfg_s3_bkt + "/cf/" + cfg["Env"] + "/"
		go func() {
			err = awss.CreateCFstack(stackName, cfS3Folder+"main.yaml", cfParams, cfTags)
			if err != nil {
				sess.Panic("ERROR @ CreateCFstack for " + stackName + ": " + err.Error())
				return
			}
			// #4.1. post deployment configs
			err = createSSMparam(stackName, cfg, awss)
			if err != nil {
				sess.Panic("post cf deployment: failed to createSSMparam: " + err.Error())
			}
			k8sCfg, err := awss.GetK8sConfigFromEks(stackName)
			if err != nil {
				sess.Panic("post cf deployment: failed to get k8sCfg for eks name: " + stackName + "err: " + err.Error())
			}
			sess.Log("&#129311; k8s.k8sCfg.Host == " + k8sCfg.Host)

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

func tco_makeCfg(r *http.Request, sess *internal.CacheBoxSessData) map[string]string {
	cfg, err := internal.ParseJsonReqBody(r.Body)
	if err != nil {
		sess.Panic("ERROR @ Unmarshal r.body, will try configs in cache, btw json.Unmarshal error = " + err.Error())
		return nil
	}
	_, ok := cfg["Region"]
	if !ok {
		sess.Log("no Region input, using default: " + internal.Cfg.DefaultRegion_aws)
		cfg["Region"] = internal.Cfg.DefaultRegion_aws
	}

	_, ok = cfg["Domain"]
	if !ok {
		sess.Panic("missing: Domain")
	}

	_, ok = cfg["DeploymentName"]
	if !ok {
		cfg["DeploymentName"] = "z"
	}

	_, ok = cfg["Env"]
	if !ok {
		sess.Log("no Env input, using dev")
		cfg["Env"] = "dev"
	}

	_, ok = cfg["cf_deploymentId"]
	if !ok {
		cfg["cf_deploymentId"] = strconv.FormatInt(time.Now().Unix()-1626102245, 36)
	}

	return cfg
}

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

func createSSMparam(stackName string, cfg map[string]string, awss *internal.AwsSvs) error {
	stacks, err := awss.GetStack(stackName)
	if err != nil {
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
			sess.Panic("ERROR @ reportCreateCFstackStatus: " + err.Error())
			return err
		}
		stack := *stacks[0]
		stackStatus = *stack.StackStatus
		sinceStart := time.Now().UTC().Sub(stack.CreationTime.UTC()).Round(time.Second).String()
		stackLink := "https://" + cfg["Region"] + ".console.aws.amazon.com/cloudformation/home?region=" + cfg["Region"] + "#/stacks/stackinfo?stackId=" + *stack.StackId
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
