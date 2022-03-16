package handlers

import (
	"encoding/json"
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
	switch r.Method {
	case "POST":
		if r.URL.Path != "/tco_gcp" {
			http.Error(w, http.StatusText(http.StatusNotFound), http.StatusNotFound)
			return
		}
		sess := internal.GetSession(r.Cookie)

		// ########## 1. get cfg from r.body ########################################
		cfg, err := turkey_makeCfg(r)
		if err != nil {
			sess.Error("ERROR @ turkey_makeCfg: " + err.Error())
			return
		}
		internal.GetLogger().Debug(fmt.Sprintf("turkeycfg: %v", cfg))
		sess.Log("[creation] started for: " + cfg.Stackname + " ... this will take a while")

		go func() {
			// ########## 2. run tf #########################################
			err = runTf(cfg, "apply")
			if err != nil {
				sess.Error("failed @runTf: " + err.Error())
				return
			}
			// ########## 3. prepare for post Deployment setups:
			// ###### get db info and complete clusterCfg (cfg)
			dbIp, err := internal.Cfg.Gcps.GetSqlPublicIp(cfg.Stackname)
			if err != nil {
				sess.Error("post tf deployment: failed to GetSqlPublicIp for: " + cfg.Stackname + ". err: " + err.Error())
				return
			}
			cfg.DB_HOST = dbIp + ":5432"
			cfg.DB_CONN = "postgres://postgres:" + cfg.DB_PASS + "@" + cfg.DB_HOST
			cfg.PSQL = "postgresql://postgres:" + cfg.DB_PASS + "@" + cfg.DB_HOST + "/ret_dev"

			internal.GetLogger().Debug("cfg.DB_HOST:" + cfg.DB_HOST + ",,, cfg.DB_CONN:" + cfg.DB_CONN + ",,, cfg.PSQL:" + cfg.PSQL)

			// ###### get k8s config
			k8sCfg, err := internal.Cfg.Gcps.GetK8sConfigFromGke(cfg.Stackname)
			if err != nil {
				sess.Error("post tf deployment: failed to get k8sCfg for eks name: " + cfg.Stackname + ". err: " + err.Error())
				return
			}
			sess.Log("&#129311; k8s.k8sCfg.Host == " + k8sCfg.Host)
			// ###### 3 produce k8s yamls
			k8sYamls, err := collectAndRenderYams_localGcp(cfg) // templated k8s yamls == yam; rendered k8s yamls == yaml
			if err != nil {
				sess.Error("failed @ collectYams: " + err.Error())
				return
			}

			// ########## 4. k8s setups
			report, err := k8sSetups(cfg, k8sCfg, k8sYamls, sess)
			if err != nil {
				sess.Error("failed @ k8sSetups: " + err.Error())
				return
			}
			// ########## what else? send an email? doe we use dns in gcp or do we keep using route53?
			//email the final manual steps to authenticated user
			clusterCfgBytes, _ := json.Marshal(cfg)
			authnUser := r.Header.Get("X-Forwarded-UserEmail")
			err = smtp.SendMail(
				internal.Cfg.SmtpServer+":"+internal.Cfg.SmtpPort,
				smtp.PlainAuth("", internal.Cfg.SmtpUser, internal.Cfg.SmtpPass, internal.Cfg.SmtpServer),
				"noreply@"+internal.Cfg.Domain,
				[]string{authnUser, "gtan@mozilla.com"},
				[]byte("To: "+authnUser+"\r\n"+
					"Subject: turkey_gcp just deployed <"+cfg.Stackname+"> \r\n"+
					"\r\n******required ******"+
					"\r\n- CNAME required: *."+cfg.Domain+" : "+report["lb"]+
					"\r\n******for https://dash."+cfg.Domain+"******"+
					"\r\n- sknoonerToken: "+report["skoonerToken"]+
					"\r\n******clusterCfg dump******"+
					"\r\n"+string(clusterCfgBytes)+
					"\r\n"),
			)
			if err != nil {
				sess.Error("failed @ email report: " + err.Error())
			}
			sess.Log("[creation] completed for (not really...still wip): " + cfg.Stackname)
		}()

		json.NewEncoder(w).Encode(map[string]interface{}{
			"stackName":    cfg.Stackname,
			"statusUpdate": "GET@/tco_gcp_status",
		})

		return

	default:
		return
		// fmt.Fprintf(w, "unexpected method: "+r.Method)
	}
})

func k8sSetups(cfg clusterCfg, k8sCfg *rest.Config, k8sYamls []string, sess *internal.CacheBoxSessData) (map[string]string, error) {

	report := make(map[string]string)

	// deploy yamls
	for _, yaml := range k8sYamls {
		err := internal.Ssa_k8sChartYaml("turkey_cluster", yaml, k8sCfg) // ServerSideApply version of kubectl apply -f
		if err != nil {
			sess.Error("post tf deployment: failed @ Ssa_k8sChartYaml" + err.Error())
			return nil, err
		}
	}

	// find sknooner token
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
	// find service-lb's host info (or public ip)
	lb, err := internal.K8s_GetServiceIngress0(k8sCfg, "ingress", "lb")
	if err != nil {
		sess.Error("post cf deployment: failed to get ingress lb's external ip because: " + err.Error())
		return nil, err
	}
	report["lb"] = lb.IP
	internal.GetLogger().Debug("~~~~~~~~~~lb: " + report["lb"])

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

var TurkeyGcp_del = http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case "POST":
		if r.URL.Path != "/tco_gcp_del" {
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
		internal.GetLogger().Debug(fmt.Sprintf("turkeycfg: %v", cfg))
		sess.Log("[deletion] started for: " + cfg.Stackname)

		go func() {
			// ######################################### 2. run tf #########################################
			err = runTf(cfg, "destroy")
			if err != nil {
				sess.Error("failed @runTf: " + err.Error())
				return
			}
			// ################# 3. delete the folder in GCS bucket for this stack
			err := internal.Cfg.Gcps.DeleteObjects("turkeycfg", "tf-backend/"+cfg.Stackname)
			if err != nil {
				sess.Error("failed @ delete tf-backend for " + cfg.Stackname + ": " + err.Error())
			}
			sess.Log("[deletion] completed for: " + cfg.Stackname)
		}()

		json.NewEncoder(w).Encode(map[string]interface{}{
			"stackName":    cfg.Stackname,
			"statusUpdate": "GET@/tco_gcp_status",
		})
		return

	default:
		return
		// fmt.Fprintf(w, "unexpected method: "+r.Method)
	}
})

func runTf(cfg clusterCfg, verb string) error {
	wd, _ := os.Getwd()
	// render the template.tf with cfg.Stackname into a Stackname named folder so that
	// 1. we can run terraform from that folder
	// 2. terraform will use a Stackname named folder in it's remote backend
	tfTemplateFile := wd + "/_files/tf/gcp.template.tf"
	tf_bin := wd + "/_files/tf/terraform"
	tfdir := wd + "/_files/tf/" + cfg.Stackname
	os.Mkdir(tfdir, os.ModePerm)

	tfFile := tfdir + "/rendered.tf"
	t, err := template.ParseFiles(tfTemplateFile)
	if err != nil {
		return err
	}
	f, _ := os.Create(tfFile)
	defer f.Close()
	t.Execute(f, struct{ ProjectId, Stackname, Region, DbUser, DbPass string }{
		ProjectId: internal.Cfg.Gcps.ProjectId,
		Stackname: cfg.Stackname,
		Region:    cfg.Region,
		DbUser:    "postgres",
		DbPass:    cfg.DB_PASS,
	})
	err = internal.RunCmd(tf_bin, "-chdir="+tfdir, "init")
	if err != nil {
		return err
	}
	// err = runCmd(tf_bin, "-chdir="+tfdir, "plan",
	// 	"-var", "project_id="+internal.Cfg.Gcps.ProjectId, "-var", "stack_id="+cfg.Stackname, "-var", "region="+cfg.Region,
	// 	"-out="+cfg.Stackname+".tfplan")
	// if err != nil {
	// 	return err
	// }
	err = internal.RunCmd(tf_bin, "-chdir="+tfdir, verb, "-auto-approve")
	if err != nil {
		return err
	}
	return nil
}
