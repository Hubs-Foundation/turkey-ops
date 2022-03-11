package handlers

import (
	"fmt"
	"net/http"
	"os"
	"text/template"

	"main/internal"
)

var gke_yams = []string{
	"cluster_00_deps.yam",
	"cluster_01_ingress.yam",
	"cluster_02_tools.yam",
	"cluster_03_turkey-services.yam",
	"cluster_04_turkey-stream.yam",
}

var TurkeyGcp = http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case "POST":
		if r.URL.Path != "/tco_gcp" {
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
		internal.GetLogger().Debug(fmt.Sprintf("turkeycfg: %v", cfg))

		// ######################################### 2. run tf #########################################
		tf_bin := "/app/_files/tf/terraform"
		tfdir := "/app/_files/tf/gcp_" + cfg.CF_deploymentId
		os.Mkdir(tfdir, os.ModePerm)
		tfTemplateFile := "/app/_files/tf/gcp.tf"
		tfFile := tfdir + "gcp.tf"
		t, err := template.ParseFiles(tfTemplateFile)
		if err != nil {
			sess.Panic(err.Error())
		}
		f, _ := os.Create(tfFile)
		defer f.Close()
		t.Execute(f, struct{ StackId string }{
			StackId: cfg.CF_deploymentId,
		})

		err = runCmd(tf_bin, "-chdir="+tfdir, "init")
		if err != nil {
			sess.Error("ERROR @ terraform init: " + err.Error())
		}
		err = runCmd(tf_bin, "-chdir="+tfdir, "plan",
			"-var", "project_id="+internal.Cfg.Gcps.ProjectId, "-var", "stack_id="+cfg.CF_deploymentId, "-var", "region="+cfg.Region,
			"-out="+cfg.CF_deploymentId+".tfplan")
		if err != nil {
			sess.Error("ERROR @ terraform plan: " + err.Error())
		}
		return
		err = runCmd(tf_bin, "-chdir="+tfdir, "apply", cfg.CF_deploymentId+".tfplan")
		if err != nil {
			sess.Error("ERROR @ terraform apply: " + err.Error())
		}
		// //aws service
		// awss, err := internal.NewAwsSvs(cfg.AWS_KEY, cfg.AWS_SECRET, cfg.AWS_REGION)
		// if err != nil {
		// 	sess.Error("ERROR @ NewAwsSvs: " + err.Error())
		// 	return
		// }
		// //test aws creds
		// accountNum, err := awss.GetAccountID()
		// if err != nil {
		// 	sess.Error("ERROR @ GetAccountID: " + err.Error())
		// 	return
		// }
		// sess.Log("good aws creds, account #: " + accountNum)
		// // ################## 1.1 see if we have a *.<domain> cert in aws (acm) ###########
		// cfg.AWS_Ingress_Cert_ARN, err = awss.ACM_findCertByDomainName("*."+cfg.Domain, "ISSUED")
		// if err != nil {
		// 	cfg.AWS_Ingress_Cert_ARN = `not-found_arn:aws:acm:<region>:<acct>:certificate/<id>`
		// } else {
		// 	sess.Log("found wildcard cert for <*." + cfg.Domain + "> in aws acm: " + cfg.AWS_Ingress_Cert_ARN)
		// }
		// // ################## 1.2 see if we have a <domain> in aws (route53) ##############
		// err = awss.Route53_addRecord("*."+cfg.Domain, "CNAME", "deploying.myhubs.net")
		// if err != nil {
		// 	sess.Log(err.Error())
		// } else {
		// 	sess.Log("found " + cfg.Domain + " in route53")
		// }

		// ######################################### 2. preps: tf params, k8sYamls ##########################
		// cfS3Folder := "https://s3.amazonaws.com/" + internal.Cfg.TurkeyCfg_s3_bkt + "/" + cfg.Env + "/cfs/"
		// cfParams := parseCFparams(map[string]string{
		// 	"deploymentId": cfg.CF_deploymentId,
		// 	"cfS3Folder":   cfS3Folder,
		// 	"turkeyDomain": cfg.Domain,
		// 	"PGpwd":        cfg.DB_PASS,
		// })
		// cfTags := []*cloudformation.Tag{
		// 	{Key: aws.String("customer-id"), Value: aws.String(r.Header.Get("X-Forwarded-UserEmail"))},
		// 	{Key: aws.String("turkeyEnv"), Value: aws.String(cfg.Env)},
		// 	{Key: aws.String("turkeyDomain"), Value: aws.String(cfg.Domain)},
		// }

		// ######################################### 3. run tf #########################################
		// go func() {
		// 	err = awss.CreateCFstack(cfg.CF_Stackname, cfS3Folder+"main.yaml", cfParams, cfTags)
		// 	if err != nil {
		// 		sess.Error("ERROR @ CreateCFstack for " + cfg.CF_Stackname + ": " + err.Error())
		// 		return
		// 	}

		// }()
		// sess.Log("&#128640;CreateCFstack started for stackName=" + cfg.CF_Stackname)
		// reportCreateCFstackStatus(cfg.CF_Stackname, cfg, sess, awss)

		// ######################################### 4. post tf configs ###################################
		// report, err := postCfConfigs(cfg, cfg.CF_Stackname, awss, r.Header.Get("X-Forwarded-UserEmail"), sess)
		// if err != nil {
		// 	sess.Error("ERROR @ postDeploymentConfigs for " + cfg.CF_Stackname + ": " + err.Error())
		// 	return
		// }
		// // ################# 4.1. update CNAME if domain's managed in route53
		// err = awss.Route53_addRecord("*."+cfg.Domain, "CNAME", report["lb"])
		// if err != nil {
		// 	sess.Log(err.Error())
		// }
		// sess.Log(" --- report for (" + cfg.CF_deploymentId + ") --- sknoonerToken: " + report["skoonerToken"])
		// sess.Log(" --- report for (" + cfg.CF_deploymentId + ") --- you must create this CNAME record manually in your nameserver {" +
		// 	cfg.Domain + ":" + report["lb"] + "}, and then go to dash." + cfg.Domain + " to view and configure cluster access")
		// sess.Log("done: " + cfg.CF_deploymentId)

		return

	default:
		return
		// fmt.Fprintf(w, "unexpected method: "+r.Method)
	}
})
