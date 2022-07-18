package handlers

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"net/smtp"
	"os"
	"strconv"
	"strings"
	"time"

	"main/ext_libs/ansihtml"

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
		tco_gcp_create(w, r)
	case "DELETE":
		tco_gcp_delete(w, r)
	case "GET":
		tco_gcp_get(w, r)
	case "PATCH":
		comp := r.URL.Query().Get("comp")
		if comp == "tf" {
			tco_gcp_tfUpdate(w, r)
		} else if comp == "k8s" {
			tco_gcp_k8sUpdate(w, r)
		}

	default:
		return
		// fmt.Fprintf(w, "unexpected method: "+r.Method)
	}
})

func tco_gcp_create(w http.ResponseWriter, r *http.Request) {
	// sess := internal.GetSession(r.Cookie)

	// ########## 1. get cfg from r.body ########################################
	cfg, err := turkey_makeCfg(r)
	if err != nil {
		internal.Logger.Error("ERROR @ turkey_makeCfg: " + err.Error())
		return
	}
	cfg.CLOUD = "gcp"
	// internal.Logger.Debug(fmt.Sprintf("turkeycfg: %v", cfg))
	internal.Logger.Info("[creation] [" + cfg.Stackname + "] " + "started ... this can take a while")

	go func() {
		// ########## 2. run tf #########################################
		tf, err := NewTfSvs(cfg.Stackname, cfg)
		if err != nil {
			internal.Logger.Error("failed @NewTfSvs : " + err.Error())
			return
		}
		tfFile, _, err := tf.Run("apply", "--auto-approve")
		if err != nil {
			internal.Logger.Error("failed @tf.Run: " + err.Error())
			return
		}
		// internal.Logger.Sugar().Debugf("tfout: %v", tfout)
		internal.Logger.Info("[creation] [" + cfg.Stackname + "] " + "tf deployment completed")
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

		internal.Logger.Info("[creation] [" + cfg.Stackname + "] " + "&#129311; GetSqlPublicIp found cfg.DB_HOST == " + cfg.DB_HOST)

		// ###### get k8s config
		k8sCfg, err := internal.Cfg.Gcps.GetK8sConfigFromGke(cfg.Stackname)
		if err != nil {
			internal.Logger.Error("[creation] [" + cfg.Stackname + "] " + "post tf deployment: failed to get k8sCfg for eks name: " + cfg.Stackname + ". err: " + err.Error())
			return
		}
		internal.Logger.Info("[creation] [" + cfg.Stackname + "]" + "&#129311; GetK8sConfigFromGke: found kubeconfig for Host == " + k8sCfg.Host)
		// ###### 3 produce k8s yamls
		k8sYamls, err := collectAndRenderYams_localGcp(cfg) // templated k8s yamls == yam; rendered k8s yamls == yaml
		if err != nil {
			internal.Logger.Error("[creation] [" + cfg.Stackname + "] " + "failed @ collectYams: " + err.Error())
			return
		}
		// upload the yamls
		for yamlIdx, yaml := range k8sYamls {
			internal.Cfg.Gcps.GCS_WriteFile("turkeycfg", "tf-backend/"+cfg.Stackname+"/k8s-yamls/"+strconv.Itoa(yamlIdx)+".yaml", yaml)
		}

		// ########## 4. k8s setups
		report, err := k8sSetups(cfg, k8sCfg, k8sYamls)
		if err != nil {
			internal.Logger.Error("[creation] [" + cfg.Stackname + "] " + "failed @ k8sSetups: " + err.Error())
			return
		}
		internal.Logger.Info("[creation] [" + cfg.Stackname + "] " + "k8sSetups completed")

		// ########## what else? send an email? doe we use dns in gcp or do we keep using route53?
		rootDomain := internal.FindRootDomain(cfg.Domain)
		err = internal.Cfg.Gcps.Dns_createRecordSet(strings.Replace(rootDomain, ".", "-", 1),
			// strings.Replace("*."+cfg.Domain, rootDomain, "", 1),
			"*."+cfg.Domain+".",
			"A", []string{report["lb"]})

		dnsMsg := "(already done in gcp/cloudDns)"
		if err != nil {
			internal.Logger.Warn("[creation] [" + cfg.Stackname + "] " + "Dns_createRecordSet failed: " + err.Error())
			dnsMsg = "root domain not found in gcp/cloud-dns, you need to create it manually"
		}

		//upload clusterCfg
		clusterCfgBytes, _ := json.Marshal(cfg)
		internal.Cfg.Gcps.GCS_WriteFile("turkeycfg", "tf-backend/"+cfg.Stackname+"/cfg.json", string(clusterCfgBytes))
		internal.Cfg.Gcps.GCS_WriteFile("turkeycfg", "tf-backend/"+cfg.Stackname+"/infra.tf", tfFile)

		//email the final manual steps to authenticated user

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
				// "\r\n******clusterCfg dump******"+
				// "\r\n"+string(clusterCfgBytes)+
				"\r\n"),
		)
		if err != nil {
			internal.Logger.Error("[creation] [" + cfg.Stackname + "] " + "failed @ email report: " + err.Error())
		}
		internal.Logger.Info("[creation] [" + cfg.Stackname + "] " + "completed for " + cfg.Stackname + ", full details emailed to " + authnUser)

	}()

	w.WriteHeader(http.StatusAccepted)
	json.NewEncoder(w).Encode(map[string]interface{}{
		"stackName":    cfg.Stackname,
		"statusUpdate": "GET@/tco_gcp_status",
	})
}

func tco_gcp_get(w http.ResponseWriter, r *http.Request) {
	bktPrefix := "tf-backend/"
	rawList, err := internal.Cfg.Gcps.GCS_List("turkeycfg", bktPrefix)
	if err != nil {
		internal.Logger.Sugar().Errorf("failed: %v", err)
	}
	clusterNames := make(map[string]bool)
	for _, ele := range rawList {
		arr := strings.Split(ele, "/")
		if len(arr) > 2 {
			clusterNames[arr[1]] = true
		}
	}
	var clusterData []map[string]string
	for k, _ := range clusterNames {
		clusterData = append(clusterData, map[string]string{
			"name":   k,
			"cfgbkt": "https://console.cloud.google.com/storage/browser/turkeycfg/tf-backend/" + k,
		})
	}

	internal.Logger.Sugar().Debugf("%v", clusterData)

	json.NewEncoder(w).Encode(map[string][]map[string]string{
		"clusters": clusterData,
	})
}
func tco_gcp_tfUpdate(w http.ResponseWriter, r *http.Request) {

	// ######################################### 1. get cfg from r.body ########################################
	cfg, err := turkey_makeCfg(r)
	if err != nil {
		internal.Logger.Error("ERROR @ turkey_makeCfg: " + err.Error())
		json.NewEncoder(w).Encode(map[string]interface{}{
			"stackName": cfg.Stackname,
			"error":     "ERROR @ turkey_makeCfg: " + err.Error(),
		})
		return
	}
	internal.Logger.Debug(fmt.Sprintf("turkeycfg: %v", cfg))

	tf, err := NewTfSvs(cfg.Stackname, cfg)
	if err != nil {
		internal.Logger.Error("failed @NewTfSvs : " + err.Error())
		return
	}

	hrInt := (time.Now().Unix() - 1648672222) / int64(time.Hour)
	tfplanFileName := cfg.Stackname + ".tf_plan." + strconv.FormatInt(hrInt, 36)
	tfplanFile := tf.Dir + "/" + tfplanFileName

	if _, err := os.Stat(tfplanFile); err != nil {
		internal.Logger.Debug("[update] [" + cfg.Stackname + "] planning: " + tfplanFileName)
		_, tfout, err := tf.Run("plan", "-out="+tfplanFileName)
		if err != nil {
			internal.Logger.Error("failed @tf.Run: " + err.Error())
			return
		}
		internal.Logger.Debug("[plan] [" + cfg.Stackname + "] completed")

		for i := 0; i < len(tfout); i++ {
			tfout[i] = strings.ReplaceAll(tfout[i], ` `, `.`)
			internal.Logger.Sugar().Debugf("<%v>", tfout[i])
			tfout[i] = string(ansihtml.ConvertToHTML([]byte(tfout[i])))
		}
		tf_out_html := strings.Join(tfout, "<br>")

		go func() {
			time.Sleep(120 * time.Minute)
			os.Remove(tfplanFile)
		}()

		json.NewEncoder(w).Encode(map[string]interface{}{
			"stackName":    cfg.Stackname,
			"msg":          "stage: planning; call again in 30min to apply",
			"tf_plan_html": tf_out_html,
		})
		return
	}

	//tfplanFileName exists == plan's already reviewed
	go func() {
		tfFile, tfout, err := runTf(cfg, "apply", "--auto-approve")
		if err != nil {
			internal.Logger.Error("failed @runTf: " + err.Error())
			return
		}
		internal.Logger.Sugar().Debugf("tfFile: %v", tfFile)
		internal.Logger.Sugar().Debugf("tfout: %v", tfout)
		internal.Logger.Debug("[tco_gcp_tfUpdate] [" + cfg.Stackname + "] " + "tf deployment completed")
		//update configs
		clusterCfgBytes, _ := json.Marshal(cfg)
		internal.Cfg.Gcps.GCS_WriteFile("turkeycfg", "tf-backend/"+cfg.Stackname+"/cfg.json", string(clusterCfgBytes))
		internal.Cfg.Gcps.GCS_WriteFile("turkeycfg", "tf-backend/"+cfg.Stackname+"/infra.tf", tfFile)
		// ###### any breaking changes?
		dbIps, err := internal.Cfg.Gcps.GetSqlIps(cfg.Stackname)
		if err != nil {
			internal.Logger.Error("[tco_gcp_tfUpdate] [" + cfg.Stackname + "] " + "post tf deployment: failed to GetSqlPublicIp . err: " + err.Error())
			return
		}
		if cfg.DB_HOST != dbIps["PRIVATE"] {
			internal.Logger.Warn(`!!! breaking change: cfg.DB_HOST != dbIps["PRIVATE"] !!!`)
		}
	}()
	json.NewEncoder(w).Encode(map[string]interface{}{
		"stackName": cfg.Stackname,
		"msg":       "stage: applying",
		"tf_plan":   tfplanFileName,
	})
	return
}
func tco_gcp_k8sUpdate(w http.ResponseWriter, r *http.Request) {

	// ######################################### 1. get cfg from r.body ########################################
	cfg, err := turkey_makeCfg(r)
	if err != nil {
		internal.Logger.Error("ERROR @ turkey_makeCfg: " + err.Error())
		json.NewEncoder(w).Encode(map[string]interface{}{
			"stackName": cfg.Stackname,
			"error":     "ERROR @ turkey_makeCfg: " + err.Error(),
		})
		return
	}
	// internal.Logger.Debug(fmt.Sprintf("turkeycfg: %v", cfg))
	http.Error(w, http.StatusText(http.StatusNotImplemented), http.StatusNotImplemented)
}

func tco_gcp_delete(w http.ResponseWriter, r *http.Request) {
	// sess := internal.GetSession(r.Cookie)

	// ######################################### 1. get cfg from r.body ########################################
	cfg, err := turkey_makeCfg(r)
	if err != nil {
		if len(cfg.Stackname) < 6 {
			json.NewEncoder(w).Encode(map[string]interface{}{
				"stackName": cfg.Stackname,
				"error":     "ERROR @ turkey_makeCfg: " + err.Error(),
			})
			return
		}
		internal.Logger.Sugar().Errorf("ERROR @ turkey_makeCfg <%v> ", err.Error())
		internal.Logger.Sugar().Warnf("continuing for stackName <%v> ", cfg.Stackname)
	}

	// internal.Logger.Debug(fmt.Sprintf("turkeycfg: %v", cfg))
	internal.Logger.Info("[deletion] [" + cfg.Stackname + "] started")

	tf, err := NewTfSvs(cfg.Stackname, cfg)
	if err != nil {
		internal.Logger.Error("failed @NewTfSvs : " + err.Error())
		return
	}

	go func() {
		// ######################################### 2. run tf #########################################
		_, _, err := tf.Run("destroy", "--auto-approve")
		if err != nil {
			internal.Logger.Error("failed @runTf: " + err.Error())
		}
		// ################# 3. delete the folder in GCS bucket for this stack
		err = internal.Cfg.Gcps.GCS_DeleteObjects("turkeycfg", "tf-backend/"+cfg.Stackname)
		if err != nil {
			internal.Logger.Error("failed @ delete tf-backend for " + cfg.Stackname + ": " + err.Error())
		}
		internal.Logger.Info("[deletion] [" + cfg.Stackname + "] completed")
	}()

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
	// internal.Logger.Debug("~~~~~~~~~~skoonerToken: " + report["skoonerToken"])
	// find service-lb's host info (or public ip)
	lb, err := internal.K8s_GetServiceIngress0(k8sCfg, "ingress", "lb")
	if err != nil {
		internal.Logger.Error("post cf deployment: failed to get ingress lb's external ip because: " + err.Error())
		return nil, err
	}
	report["lb"] = lb.IP
	internal.Logger.Info("~~~~~~~~~~lb: " + report["lb"])

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
