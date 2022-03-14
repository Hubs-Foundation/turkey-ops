package handlers

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
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
		cfg, err := turkey_makeCfg(r)
		if err != nil {
			sess.Error("ERROR @ turkey_makeCfg: " + err.Error())
			return
		}
		internal.GetLogger().Debug(fmt.Sprintf("turkeycfg: %v", cfg))
		sess.Log("[creation] started for: " + cfg.Stackname + " ... this will take a while")

		go func() {
			// ######################################### 2. run tf #########################################
			err = runTf(cfg, "apply")
			if err != nil {
				sess.Error("failed @runTf: " + err.Error())
				return
			}
			// ########## 3. get into gke, render the yamls from yams and "kubectl apply -f" them  #########
			// ###### 3.1 get k8s config
			k8sCfg, err := internal.Cfg.Gcps.GetK8sConfigFromGke(cfg.Stackname)
			if err != nil {
				sess.Error("post tf deployment: failed to get k8sCfg for eks name: " + cfg.Stackname + ". err: " + err.Error())
				return
			}
			sess.Log("&#129311; k8s.k8sCfg.Host == " + k8sCfg.Host)
			// nsList, err := internal.K8s_getNs(k8sCfg)
			// if err != nil {
			// 	sess.Error("failed @K8s_getNs: " + err.Error())
			// 	return
			// }
			// sess.Log(fmt.Sprintf("good k8sCfg: %v", nsList.Items))
			// ###### 3.2 render + deploy yamls
			k8sYamls, err := collectAndRenderYams_localGcp(cfg) // templated k8s yamls == yam; rendered k8s yamls == yaml
			if err != nil {
				sess.Error("failed @ collectYams: " + err.Error())
				return
			}
			for _, yaml := range k8sYamls {
				err := internal.Ssa_k8sChartYaml("turkey_cluster", yaml, k8sCfg) // ServerSideApply version of kubectl apply -f
				if err != nil {
					sess.Error("post tf deployment: failed @ Ssa_k8sChartYaml" + err.Error())
					return
				}
			}
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

func collectAndRenderYams_localGcp(cfg clusterCfg) ([]string, error) {
	yamFiles, _ := ioutil.ReadDir("./yamls/gcp/")
	var yams []string
	for _, f := range yamFiles {
		yam, _ := ioutil.ReadFile("./yamls/gcp/" + f.Name())
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
	internal.GetLogger().Debug("~~~~~~os.Getwd()~~~~~~~" + wd)
	// render the template.tf with cfg.Stackname into a Stackname named folder so that
	// 1. we can run terraform from that folder
	// 2. terraform will use a Stackname named folder in it's remote backend
	tfTemplateFile := "/app/_files/tf/gcp.template.tf"
	tf_bin := "/app/_files/tf/terraform"
	tfdir := "/app/_files/tf/" + cfg.Stackname
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
