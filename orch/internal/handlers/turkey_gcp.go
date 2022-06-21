package handlers

import (
	"encoding/json"
	"errors"
	"fmt"
	"io/ioutil"
	"net/http"
	"net/smtp"
	"os"
	"strings"
	"text/template"

	"main/internal"

	"k8s.io/client-go/rest"
)

var (
	gcp_yams_dir = "./_files/yams/gcp/"
)

var TurkeyGcp = http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
	if r.URL.Path != "/tco_gcp" {
		http.Error(w, http.StatusText(http.StatusNotFound), http.StatusNotFound)
		return
	}

	switch r.Method {
	case "POST":
		tcp_gcp_create(w, r)
	case "DELETE":
		tcp_gcp_delete(w, r)

	default:
		return
		// fmt.Fprintf(w, "unexpected method: "+r.Method)
	}
})

func tcp_gcp_create(w http.ResponseWriter, r *http.Request) {
	// sess := internal.GetSession(r.Cookie)

	// ########## 1. get cfg from r.body ########################################
	cfg, err := turkey_makeCfg(r)
	if err != nil {
		internal.Logger.Error("ERROR @ turkey_makeCfg: " + err.Error())
		return
	}
	cfg.CLOUD = "gcp"
	internal.Logger.Debug(fmt.Sprintf("turkeycfg: %v", cfg))
	internal.Logger.Debug("[creation] [" + cfg.Stackname + "] " + "started ... this can take a while")

	go func() {
		// ########## 2. run tf #########################################
		err := runTf(cfg, "apply")
		if err != nil {
			internal.Logger.Error("failed @runTf: " + err.Error())
			return
		}
		internal.Logger.Debug("[creation] [" + cfg.Stackname + "] " + "tf deployment completed")
		// ########## 3. prepare for post Deployment setups:
		// ###### get db info and complete clusterCfg (cfg)
		dbIps, err := internal.Cfg.Gcps.GetSqlIps(cfg.Stackname)
		if err != nil {
			internal.Logger.Error("[creation] [" + cfg.Stackname + "] " + "post tf deployment: failed to GetSqlPublicIp . err: " + err.Error())
			return
		}

		cfg.DB_HOST = dbIps["PRIVATE"] //+ ":5432"
		cfg.DB_CONN = "postgres://postgres:" + cfg.DB_PASS + "@" + cfg.DB_HOST
		cfg.PSQL = "postgresql://postgres:" + cfg.DB_PASS + "@" + cfg.DB_HOST + "/ret_dev"

		internal.Logger.Debug("[creation] [" + cfg.Stackname + "] " + "&#129311; GetSqlPublicIp found cfg.DB_HOST == " + cfg.DB_HOST)

		// ###### get k8s config
		k8sCfg, err := internal.Cfg.Gcps.GetK8sConfigFromGke(cfg.Stackname)
		if err != nil {
			internal.Logger.Error("[creation] [" + cfg.Stackname + "] " + "post tf deployment: failed to get k8sCfg for eks name: " + cfg.Stackname + ". err: " + err.Error())
			return
		}
		internal.Logger.Debug("[creation] [" + cfg.Stackname + "]" + "&#129311; GetK8sConfigFromGke: found kubeconfig for Host == " + k8sCfg.Host)
		// ###### 3 produce k8s yamls
		k8sYamls, err := collectAndRenderYams_localGcp(cfg) // templated k8s yamls == yam; rendered k8s yamls == yaml
		if err != nil {
			internal.Logger.Error("[creation] [" + cfg.Stackname + "] " + "failed @ collectYams: " + err.Error())
			return
		}

		// ########## 4. k8s setups
		report, err := k8sSetups(cfg, k8sCfg, k8sYamls)
		if err != nil {
			internal.Logger.Error("[creation] [" + cfg.Stackname + "] " + "failed @ k8sSetups: " + err.Error())
			return
		}
		internal.Logger.Debug("[creation] [" + cfg.Stackname + "] " + "k8sSetups completed")

		// ########## what else? send an email? doe we use dns in gcp or do we keep using route53?
		rootDomain := internal.RootDomain(cfg.Domain)
		err = internal.Cfg.Gcps.Dns_createRecordSet(strings.Replace(rootDomain, ".", "-", 1),
			// strings.Replace("*."+cfg.Domain, rootDomain, "", 1),
			"*."+cfg.Domain+".",
			"A", []string{report["lb"]})

		dnsMsg := "(already done in gcp/cloudDns)"
		if err != nil {
			internal.Logger.Debug("[creation] [" + cfg.Stackname + "] " + "Dns_createRecordSet failed: " + err.Error())
			dnsMsg = "root domain not found in gcp/cloud-dns, you need to create it manually"
		}

		//email the final manual steps to authenticated user
		clusterCfgBytes, _ := json.Marshal(cfg)
		authnUser := r.Header.Get("X-Forwarded-UserEmail")
		err = smtp.SendMail(
			internal.Cfg.SmtpServer+":"+internal.Cfg.SmtpPort,
			smtp.PlainAuth("", internal.Cfg.SmtpUser, internal.Cfg.SmtpPass, internal.Cfg.SmtpServer),
			"noreply@"+internal.Cfg.Domain,
			[]string{authnUser, "gtan@mozilla.com"},
			[]byte("To: "+authnUser+"\r\n"+
				"Subject: turkey_gcp just deployed <"+cfg.Stackname+"> for <"+cfg.Domain+"> \r\n"+
				"\r\n******required ("+dnsMsg+")******"+
				"\r\n- dns record required: *."+cfg.Domain+" : "+report["lb"]+
				"\r\n******for https://dash."+cfg.Domain+"******"+
				"\r\n- sknoonerToken: "+report["skoonerToken"]+
				"\r\n******get: kubeconfig******"+
				"\r\n - gcloud container clusters get-credentials --region us-east1 "+cfg.Stackname+
				"\r\n******get: cluster https cert******"+
				"\r\n - kubectl -n ingress get secret letsencrypt -o yaml"+
				"\r\n******clusterCfg dump******"+
				"\r\n"+string(clusterCfgBytes)+
				"\r\n"),
		)
		if err != nil {
			internal.Logger.Error("[creation] [" + cfg.Stackname + "] " + "failed @ email report: " + err.Error())
		}
		internal.Logger.Debug("[creation] [" + cfg.Stackname + "] " + "completed for " + cfg.Stackname + ", full details emailed to " + authnUser)

	}()

	w.WriteHeader(http.StatusAccepted)
	json.NewEncoder(w).Encode(map[string]interface{}{
		"stackName":    cfg.Stackname,
		"statusUpdate": "GET@/tco_gcp_status",
	})
}

func k8sSetups(cfg clusterCfg, k8sCfg *rest.Config, k8sYamls []string) (map[string]string, error) {

	report := make(map[string]string)

	// deploy yamls
	for _, yaml := range k8sYamls {
		err := internal.Ssa_k8sChartYaml("turkey_cluster", yaml, k8sCfg) // ServerSideApply version of kubectl apply -f
		if err != nil {
			internal.Logger.Error("post tf deployment: failed @ Ssa_k8sChartYaml" + err.Error())
			return nil, err
		}
	}

	// find sknooner token
	toolsSecrets, err := internal.K8s_GetAllSecrets(k8sCfg, "tools")
	if err != nil {
		internal.Logger.Error("post cf deployment: failed to get k8s secrets in tools namespace because: " + err.Error())
		return nil, err
	}
	for k, v := range toolsSecrets {
		if strings.HasPrefix(k, "skooner-sa-token-") {
			report["skoonerToken"] = string(v["token"])
		}
	}
	internal.Logger.Debug("~~~~~~~~~~skoonerToken: " + report["skoonerToken"])
	// find service-lb's host info (or public ip)
	lb, err := internal.K8s_GetServiceIngress0(k8sCfg, "ingress", "lb")
	if err != nil {
		internal.Logger.Error("post cf deployment: failed to get ingress lb's external ip because: " + err.Error())
		return nil, err
	}
	report["lb"] = lb.IP
	internal.Logger.Debug("~~~~~~~~~~lb: " + report["lb"])

	return report, nil
}

func collectAndRenderYams_localGcp(cfg clusterCfg) ([]string, error) {
	yamFiles, _ := ioutil.ReadDir(gcp_yams_dir)
	var yams []string
	for _, f := range yamFiles {
		yam, _ := ioutil.ReadFile(gcp_yams_dir + f.Name())
		yams = append(yams, string(yam))
	}
	yamls, err := internal.K8s_render_yams(yams, cfg)
	if err != nil {
		return nil, err
	}
	return yamls, nil
}

func tcp_gcp_delete(w http.ResponseWriter, r *http.Request) {
	// sess := internal.GetSession(r.Cookie)

	// ######################################### 1. get cfg from r.body ########################################
	cfg, err := turkey_makeCfg(r)
	if err != nil {
		internal.Logger.Error("ERROR @ turkey_makeCfg: " + err.Error())
		return
	}
	internal.Logger.Debug(fmt.Sprintf("turkeycfg: %v", cfg))
	internal.Logger.Debug("[deletion] [" + cfg.Stackname + "] started")

	go func() {
		// ######################################### 2. run tf #########################################
		err := runTf(cfg, "destroy")
		if err != nil {
			internal.Logger.Error("failed @runTf: " + err.Error())
			return
		}
		// ################# 3. delete the folder in GCS bucket for this stack
		err = internal.Cfg.Gcps.DeleteObjects("turkeycfg", "tf-backend/"+cfg.Stackname)
		if err != nil {
			internal.Logger.Error("failed @ delete tf-backend for " + cfg.Stackname + ": " + err.Error())
		}
		internal.Logger.Debug("[deletion] [" + cfg.Stackname + "] completed")
	}()

	json.NewEncoder(w).Encode(map[string]interface{}{
		"stackName":    cfg.Stackname,
		"statusUpdate": "GET@/tco_gcp_status",
	})
}

func runTf(cfg clusterCfg, verb string) error {
	wd, _ := os.Getwd()
	// render the template.tf with cfg.Stackname into a Stackname named folder so that
	// 1. we can run terraform from that folder
	// 2. terraform will use a Stackname named folder in it's remote backend
	tfTemplateFile := wd + "/_files/tf/gcp.tf.gotemplate"
	if _, err := os.Stat(tfTemplateFile); errors.Is(err, os.ErrNotExist) {
		return err
	}

	// tf_bin := wd + "/_files/tf/terraform"
	tf_bin := "terraform"
	tfdir := wd + "/_files/tf/" + cfg.Stackname
	os.Mkdir(tfdir, os.ModePerm)

	tfFile := tfdir + "/rendered.tf"
	t, err := template.ParseFiles(tfTemplateFile)
	if err != nil {
		return err
	}
	f, _ := os.Create(tfFile)
	defer f.Close()

	t.Execute(f, struct{ ProjectId, Stackname, Region, DbUser, DbPass, Env string }{
		ProjectId: internal.Cfg.Gcps.ProjectId,
		Stackname: cfg.Stackname,
		Region:    cfg.Region,
		DbUser:    "postgres",
		DbPass:    cfg.DB_PASS,
		Env:       cfg.Env,
	})

	err = internal.RunCmd_sync(tf_bin, "-chdir="+tfdir, "init")
	if err != nil {
		tfBytes, _ := ioutil.ReadFile(tfFile)
		return errors.New(err.Error() + "...cat $tfFile: " + string(tfBytes))
	}
	// err = runCmd(tf_bin, "-chdir="+tfdir, "plan",
	// 	"-var", "project_id="+internal.Cfg.Gcps.ProjectId, "-var", "stack_id="+cfg.Stackname, "-var", "region="+cfg.Region,
	// 	"-out="+cfg.Stackname+".tfplan")
	// if err != nil {
	// 	return err
	// }
	err = internal.RunCmd_sync(tf_bin, "-chdir="+tfdir, verb, "-auto-approve")

	if err != nil {
		return err
	}
	return nil
}
