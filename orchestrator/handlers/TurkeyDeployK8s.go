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
	"strings"
	"text/template"

	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/serializer/yaml"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/discovery"
	"k8s.io/client-go/discovery/cached/memory"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/restmapper"
)

type yamlCfg struct {
	UserId    string
	Subdomain string
	Domain    string
}

var TurkeyDeployK8s = http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case "POST":
		if r.URL.Path != "/TurkeyDeployK8s" {
			http.Error(w, http.StatusText(http.StatusNotFound), http.StatusNotFound)
			return
		}

		sess := utils.GetSession(r.Cookie)

		//get r.body
		rBodyBytes, err := ioutil.ReadAll(r.Body)
		if err != nil {
			sess.PushMsg("ERROR @ reading r.body, error = " + err.Error())
			return
		}

		if string(rBodyBytes) != "fkzXYeGRjjryynH23upDQK3584vG8SmE" {
			return
		}
		sess.PushMsg("hello")
		//try to get k8s config from r.body
		// cfg, err := clientcmd.RESTConfigFromKubeConfig(rBodyBytes)
		// if err != nil {

		//getting yamlCfgs in query params
		_userid, found := r.URL.Query()["userid"]
		if !found || len(_userid) != 1 {
			panic("bad <userid> in query parameters")
		}
		turkeyUserId := _userid[0]

		_subdomain, found := r.URL.Query()["subdomain"]
		if !found || len(_subdomain) != 1 {
			panic("bad <subdomain> in query parameters")
		}
		turkeySubdomain := _subdomain[0]

		turkeyDomain := defaultTurkeyDomain
		_domain, found := r.URL.Query()["domain"]
		if found && len(_domain) == 1 {
			turkeyDomain = _domain[0]
		}

		//render turkey-k8s-chart by apply yamlCfgs to turkey.yam
		t, err := template.ParseFiles("./_files/turkey.yam")
		if err != nil {
			panic(err.Error())
		}
		_data := yamlCfg{
			UserId:    turkeyUserId,
			Subdomain: turkeySubdomain,
			Domain:    turkeyDomain,
		}
		var buf bytes.Buffer
		t.Execute(&buf, _data)
		k8sChartYaml := buf.String()
		//<debugging>
		if turkeyUserId == "_r.dump" {
			headerBytes, _ := json.Marshal(r.Header)
			sess.PushMsg(string(headerBytes))
			cookieMap := make(map[string]string)
			for _, c := range r.Cookies() {
				cookieMap[c.Name] = c.Value
			}
			cookieJson, _ := json.Marshal(cookieMap)
			sess.PushMsg(string(cookieJson))

			return
		}
		if turkeyUserId == "_gimmechart" {
			w.Header().Set("Content-Disposition", "attachment; filename="+turkeySubdomain+".yaml")
			w.Header().Set("Content-Type", "text/plain")
			io.Copy(w, strings.NewReader(k8sChartYaml))
			return
		}
		//</debugging>

		sess.PushMsg("&#9989; ... using InClusterConfig")
		cfg, err := rest.InClusterConfig()
		// }
		if cfg == nil {
			sess.PushMsg("ERROR" + err.Error())
			panic(err.Error())
		}
		sess.PushMsg("&#129311; k8s.cfg.Host == " + cfg.Host)

		//basically kubectl apply -f
		sess.PushMsg("&#128640;[DEBUG] --- deployment started")
		err = ssa_k8sChartYaml(turkeyUserId, k8sChartYaml, cfg)
		if err != nil {
			sess.PushMsg("ERROR --- deployment FAILED !!! because" + fmt.Sprint(err))
			panic(err.Error())
		}
		skipadminLink := "https://" + turkeySubdomain + "." + turkeyDomain + "?skipadmin"
		sess.PushMsg("&#128640;[DEBUG] --- deployment completed for: <a href=\"" +
			skipadminLink + "\" target=\"_blank\"><b>&#128279;" + turkeyUserId + "'s " + turkeySubdomain + "</b></a>")

		// clientset, err := kubernetes.NewForConfig(cfg)
		// if err != nil {
		// 	panic(err.Error())
		// }
		// nsList, err := clientset.CoreV1().Namespaces().List(context.TODO(), metav1.ListOptions{})
		// if err != nil {
		// 	panic(err.Error())
		// }
		// for _, ns := range nsList.Items {
		// 	sess.PushMsg("~~~~~ns~~~~~~" + ns.ObjectMeta.Name)
		// }

		return

	default:
		return
		// fmt.Fprintf(w, "unexpected method: "+r.Method)
	}
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
