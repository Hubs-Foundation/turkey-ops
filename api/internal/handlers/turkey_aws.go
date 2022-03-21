package handlers

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/smtp"
	"strconv"
	"strings"
	"time"

	"main/internal"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/service/cloudformation"
	"github.com/aws/aws-sdk-go/service/ec2"
	"github.com/aws/aws-sdk-go/service/eks"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
)

var eks_yams = []string{
	"cluster_00_deps.yam",
	"cluster_01_ingress.yam",
	"cluster_02_tools.yam",
	"cluster_03_turkey-services.yam",
	"cluster_04_turkey-stream.yam",
	"cluster_05_monitoring.yam",
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
		cfg, err := turkey_makeCfg(r)
		if err != nil {
			sess.Error("ERROR @ turkey_makeCfg: " + err.Error())
			return
		}
		cfg.CLOUD = "aws"
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
		go func() {
			// ######################################### 2. run cloudformation #########################################
			// ########################## 2.1 preps: cf params ##########################
			cfS3Folder := "https://s3.amazonaws.com/" + internal.Cfg.TurkeyCfg_s3_bkt + "/" + cfg.Env + "/cfs/"
			cfParams := parseCFparams(map[string]string{
				// "deploymentId": cfg.deploymentId,
				"cfS3Folder":   cfS3Folder,
				"turkeyDomain": cfg.Domain,
				"PGpwd":        cfg.DB_PASS,
			})
			cfTags := []*cloudformation.Tag{
				{Key: aws.String("customer-id"), Value: aws.String(r.Header.Get("X-Forwarded-UserEmail"))},
				{Key: aws.String("turkeyEnv"), Value: aws.String(cfg.Env)},
				{Key: aws.String("turkeyDomain"), Value: aws.String(cfg.Domain)},
			}
			// ########################## 2.2 run cloudformation ##########################
			go func() {
				err = awss.CreateCFstack(cfg.Stackname, cfS3Folder+"main.yaml", cfParams, cfTags)
				if err != nil {
					sess.Error("ERROR @ CreateCFstack for " + cfg.Stackname + ": " + err.Error())
					return
				}

			}()
			sess.Log("&#128640;CreateCFstack started for stackName=" + cfg.Stackname)
			reportCreateCFstackStatus(cfg.Stackname, cfg, sess, awss)
			// ######################################### 3. post cloudformation configs ###################################
			report, err := postCfConfigs(cfg, cfg.Stackname, awss, r.Header.Get("X-Forwarded-UserEmail"), sess)
			if err != nil {
				sess.Error("ERROR @ postDeploymentConfigs for " + cfg.Stackname + ": " + err.Error())
				return
			}
			// ################# 3.1. update CNAME if domain's managed in route53
			err = awss.Route53_addRecord("*."+cfg.Domain, "CNAME", report["lb"])
			if err != nil {
				sess.Log(err.Error())
			}
			sess.Log(" --- report for (" + cfg.Stackname + ") --- sknoonerToken: " + report["skoonerToken"])
			sess.Log(" --- report for (" + cfg.Stackname + ") --- you must create this CNAME record manually in your nameserver {" +
				cfg.Domain + ":" + report["lb"] + "}, and then go to dash." + cfg.Domain + " to view and configure cluster access")
			sess.Log("done: " + cfg.Stackname)
		}()

		json.NewEncoder(w).Encode(map[string]interface{}{
			"stackName":    cfg.Stackname,
			"statusUpdate": "GET@/tco_aws_status",
		})
		return

	default:
		return
		// fmt.Fprintf(w, "unexpected method: "+r.Method)
	}
})

func postCfConfigs(cfg clusterCfg, stackName string, awss *internal.AwsSvs, authnUser string, sess *internal.CacheBoxSessData) (map[string]string, error) {
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
	k8sYamls, err := collectAndRenderYams(cfg.Env, awss, cfg) // templated k8s yamls == yam; rendered k8s yamls == yaml
	if err != nil {
		sess.Error("failed @ collectYams: " + err.Error())
		return nil, err
	}
	for _, yaml := range k8sYamls {
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
	lb, err := internal.K8s_GetServiceIngress0(k8sCfg, "ingress", "lb")
	if err != nil {
		sess.Error("post cf deployment: failed to get ingress lb's external ip because: " + err.Error())
		return nil, err
	}
	report["lb"] = lb.Hostname
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
			"Subject: turkey_aws just deployed <"+stackName+"> \r\n"+
			"\r\n******required******"+
			"\r\n- CNAME required: *."+cfg.Domain+" : "+report["lb"]+
			"\r\n******optionals******"+
			"\r\n- update https cert at: https://dash."+cfg.Domain+"/#!service/ingress/lb"+
			"\r\n- for aws-eks, update role mappings at: https://dash."+cfg.Domain+"/#!configmap/kube-system/aws-auth"+
			"\r\n******for https://dash."+cfg.Domain+"******"+
			"\r\n- sknoonerToken: "+report["skoonerToken"]+
			"\r\n******clusterCfg dump******"+
			"\r\n"+string(clusterCfgBytes)+
			"\r\n"),
	)
	if err != nil {
		internal.GetLogger().Error(err.Error())
	}

	return report, nil
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

func collectAndRenderYams(env string, awss *internal.AwsSvs, cfg clusterCfg) ([]string, error) {
	var yams []string

	internal.GetLogger().Debug(fmt.Sprintf("%v", eks_yams))

	for _, s3Key := range eks_yams {
		yamS3 := env + "/yams/" + s3Key
		yam, err := awss.S3Download_string(internal.Cfg.TurkeyCfg_s3_bkt, yamS3)
		if err != nil {
			return yams, err
		}
		yams = append(yams, yam)
	}
	internal.GetLogger().Debug(fmt.Sprintf("len(yams) = %v", len(yams)))

	yamls, err := internal.K8s_render_yams(yams, cfg)
	if err != nil {
		return nil, err
	}

	return yamls, nil
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
	//	2.1 get launchtemplate id from nodegroup
	eksClient := eks.New(as.Sess)
	ng, err := eksClient.DescribeNodegroup(&eks.DescribeNodegroupInput{
		ClusterName:   &stackName,
		NodegroupName: aws.String(stackName + "-ng"),
	})
	if err != nil {
		return err
	}
	//	2.2 get ami-id from launchtemplateVersion
	ec2Client := ec2.New(as.Sess)
	ltvs, err := ec2Client.DescribeLaunchTemplateVersions(&ec2.DescribeLaunchTemplateVersionsInput{
		LaunchTemplateId: ng.Nodegroup.LaunchTemplate.Id,
		Versions:         []*string{aws.String("1")},
	})
	if err != nil {
		return err
	}
	if len(ltvs.LaunchTemplateVersions) < 1 {
		return errors.New("len(ltvs.LaunchTemplateVersions) < 1")
	}
	// 2.3 this will create a new fake version (version: 2)
	r2, err := ec2Client.CreateLaunchTemplateVersion(&ec2.CreateLaunchTemplateVersionInput{
		LaunchTemplateId: ng.Nodegroup.LaunchTemplate.Id,
		LaunchTemplateData: &ec2.RequestLaunchTemplateData{
			ImageId: ltvs.LaunchTemplateVersions[0].LaunchTemplateData.ImageId, //required, why is this not copied from SourceVersion???
		},
		SourceVersion:      aws.String("1"),
		VersionDescription: aws.String("same-as-version-1"),
	})
	internal.GetLogger().Sugar().Debugf("ec2Client.CreateLaunchTemplateVersion ==> %v", *r2)
	if err != nil {
		return err
	}
	// 3. update the nodegroup with new launchTemplate
	r3, err := eksClient.UpdateNodegroupVersion(&eks.UpdateNodegroupVersionInput{
		ClusterName:   &stackName,
		NodegroupName: ng.Nodegroup.NodegroupName,
		LaunchTemplate: &eks.LaunchTemplateSpecification{
			Id:      ng.Nodegroup.LaunchTemplate.Id,
			Version: aws.String("2"),
		},
	})
	internal.GetLogger().Sugar().Debugf("eksClient.UpdateNodegroupVersion ==> %v", *r3)
	if err != nil {
		return err
	}

	return nil
}

//////////////////////////////////////////////////////////////////////////////////
