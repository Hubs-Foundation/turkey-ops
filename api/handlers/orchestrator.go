package handlers

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"io/ioutil"
	"main/utils"
	"net/http"
	"strconv"
	"strings"
	"text/template"
	"time"

	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/serializer/yaml"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/discovery"
	"k8s.io/client-go/discovery/cached/memory"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/restmapper"
)

type turkeyCfg struct {
	Key       string `json:"key"`
	TurkeyId  string `json:"turkeyid"`
	Subdomain string `json:"subdomain"`
	Domain    string `json:"domain"`
	Tier      string `json:"tier"`
	UserEmail string `json:"useremail"`
}

var Hc_deploy = http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {

	if r.URL.Path != "/hc_deploy" || r.Method != "POST" {
		http.Error(w, http.StatusText(http.StatusNotFound), http.StatusNotFound)
		return
	}

	sess := utils.GetSession(r.Cookie)

	cfg, err := makeCfg(r)
	if err != nil {
		sess.PushMsg("bad turkeyCfg: " + err.Error())
	}

	// // tmp -- until authZ in place
	// if cfg.Key != "fkzXYeGRjjryynH23upDQK3584vG8SmE" {
	// 	sess.PushMsg("bad turkeyCfg.Key")
	// 	return
	// }

	// userid is required
	if cfg.TurkeyId == "" {
		sess.PushMsg("ERROR bad turkeyCfg.UserId")
		return
	}
	// domain is required
	if cfg.Domain == "" {
		sess.PushMsg("ERROR bad turkeyCfg.Domain")
		return
	}

	//default Tier is free
	if cfg.Tier == "" {
		cfg.Tier = "free"
	}
	//default Subdomain is a string hashed from turkeyId and time
	if cfg.Subdomain == "" {
		cfg.Subdomain = cfg.TurkeyId + "-" + strconv.FormatInt(time.Now().Unix()-1626102245, 36)
	}

	cfg.UserEmail = r.Header.Get("X-Forwarded-UserEmail")
	if cfg.UserEmail == "" {
		sess.PushMsg("failed to get cfg.UserEmail")
		return
	}

	//render turkey-k8s-chart by apply cfg to turkey.yam
	t, err := template.ParseFiles("./_files/turkey.yam")
	if err != nil {
		panic(err.Error())
	}
	var buf bytes.Buffer
	t.Execute(&buf, cfg)
	k8sChartYaml := buf.String()

	//getting k8s config
	sess.PushMsg("&#9989; ... using InClusterConfig")
	k8sCfg, err := rest.InClusterConfig()
	// }
	if k8sCfg == nil {
		sess.PushMsg("ERROR" + err.Error())
		panic(err.Error())
	}
	sess.PushMsg("&#129311; k8s.k8sCfg.Host == " + k8sCfg.Host)

	// kubectl apply -f <file.yaml> --server-side --field-manager "turkey-userid-<cfg.UserId>"
	sess.PushMsg("&#128640;[DEBUG] --- deployment started")
	err = ssa_k8sChartYaml(cfg.TurkeyId, k8sChartYaml, k8sCfg)
	if err != nil {
		sess.PushMsg("ERROR --- deployment FAILED !!! because" + fmt.Sprint(err))
		panic(err.Error())
	}
	skipadminLink := "https://" + cfg.Subdomain + "." + cfg.Domain + "?skipadmin"
	sess.PushMsg("&#128640;[DEBUG] --- deployment completed for: <a href=\"" +
		skipadminLink + "\" target=\"_blank\"><b>&#128279;" + cfg.TurkeyId + "'s " + cfg.Subdomain + "</b></a>")

	return

})

var decUnstructured = yaml.NewDecodingSerializer(unstructured.UnstructuredJSONScheme)

func ssa_k8sChartYaml(userId, k8sChartYaml string, cfg *rest.Config) error {
	// Prepare a RESTMapper to find GVR
	dc, err := discovery.NewDiscoveryClientForConfig(cfg)
	if err != nil {
		return err
	}
	mapper := restmapper.NewDeferredDiscoveryRESTMapper(memory.NewMemCacheClient(dc))
	// Prepare the dynamic client
	dyn, err := dynamic.NewForConfig(cfg)
	if err != nil {
		return err
	}
	for _, k8sYaml := range strings.Split(k8sChartYaml, "\n---\n") {
		// fmt.Println("\n\n\n@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@\n")
		// fmt.Println(k8sYaml)
		// fmt.Println("\n@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@\n\n\n")
		// continue

		// Decode YAML manifest into unstructured.Unstructured
		obj := &unstructured.Unstructured{}
		_, gvk, err := decUnstructured.Decode([]byte(k8sYaml), nil, obj)
		if err != nil {
			return err
		}
		// Find GVR
		mapping, err := mapper.RESTMapping(gvk.GroupKind(), gvk.Version)
		if err != nil {
			return err
		}
		// Obtain REST interface for the GVR
		var dr dynamic.ResourceInterface
		if mapping.Scope.Name() == meta.RESTScopeNameNamespace {
			// namespaced resources should specify the namespace
			dr = dyn.Resource(mapping.Resource).Namespace(obj.GetNamespace())
		} else {
			// for cluster-wide resources
			dr = dyn.Resource(mapping.Resource)
		}
		// Marshal object into JSON
		data, err := json.Marshal(obj)
		if err != nil {
			return err
		}
		// Create or Update the object with SSA // types.ApplyPatchType indicates SSA. // FieldManager specifies the field owner ID.
		_, err = dr.Patch(context.TODO(),
			obj.GetName(), types.ApplyPatchType, data,
			metav1.PatchOptions{
				FieldManager: "turkey-userid-" + userId,
			})
		if err != nil {
			return err
		}
	}
	return err
}

var Hc_get = http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
	if r.URL.Path != "/hc_get" || r.Method != "POST" {
		http.Error(w, http.StatusText(http.StatusNotFound), http.StatusNotFound)
		return
	}
	sess := utils.GetSession(r.Cookie)
	cfg, err := makeCfg(r)
	if err != nil {
		sess.PushMsg("bad turkeyCfg: " + err.Error())
		return
	}

	//<debugging cheatcodes>
	if cfg.TurkeyId[0:4] == "dev_" {
		if cfg.Subdomain != "dev0" {
			fmt.Println("dev_ cheatcodes only work with subdomain == dev0 ")
			return
		}
		sess.PushMsg(`turkeyUserId[0:4] == dev_ means dev mode`)

		cfg.UserEmail = "foo@bar.com"
		t, _ := template.ParseFiles("./_files/turkey.yam")
		var buf bytes.Buffer
		t.Execute(&buf, cfg)
		k8sChartYaml := buf.String()
		if cfg.TurkeyId == "dev_dumpr" {
			sess.PushMsg(dumpHeader(r))
			return
		}
		if cfg.TurkeyId == "dev_gimmechart" {
			w.Header().Set("Content-Disposition", "attachment; filename="+cfg.Subdomain+".yaml")
			w.Header().Set("Content-Type", "text/plain")
			io.Copy(w, strings.NewReader(k8sChartYaml))
			return
		}
	}
	//</debugging cheatcodes>

	//getting k8s config
	sess.PushMsg("&#9989; ... using InClusterConfig")
	k8sCfg, err := rest.InClusterConfig()
	// }
	if k8sCfg == nil {
		sess.PushMsg("ERROR" + err.Error())
		panic(err.Error())
	}
	sess.PushMsg("&#129311; k8s.k8sCfg.Host == " + k8sCfg.Host)
	clientset, err := kubernetes.NewForConfig(k8sCfg)
	if err != nil {
		panic(err.Error())
	}
	nsList, err := clientset.CoreV1().Namespaces().List(context.TODO(),
		metav1.ListOptions{
			LabelSelector: "UserId=" + cfg.TurkeyId,
		})
	if err != nil {
		panic(err.Error())
	}
	sess.PushMsg("GET --- user <" + cfg.TurkeyId + "> owns: ")
	for _, ns := range nsList.Items {
		sess.PushMsg("......<" + ns.ObjectMeta.Name + ">")
	}
})

func makeCfg(r *http.Request) (turkeyCfg, error) {
	var cfg turkeyCfg

	//get r.body
	rBodyBytes, err := ioutil.ReadAll(r.Body)
	if err != nil {
		fmt.Println("ERROR @ reading r.body, error = " + err.Error())
		return cfg, err
	}
	//make cfg
	err = json.Unmarshal(rBodyBytes, &cfg)
	if err != nil {
		fmt.Println("bad turkeyCfg: " + string(rBodyBytes))
		return cfg, err
	}
	return cfg, nil
}
