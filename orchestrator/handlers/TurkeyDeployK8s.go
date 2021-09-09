package handlers

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
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
	"k8s.io/client-go/tools/clientcmd"
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
		sess.PushMsg("hello")

		//get r.body
		rBodyBytes, err := ioutil.ReadAll(r.Body)
		if err != nil {
			sess.PushMsg("ERROR @ reading r.body, error = " + err.Error())
			return
		}

		//try to get k8s config from r.body
		cfg, err := clientcmd.RESTConfigFromKubeConfig(rBodyBytes)
		if err != nil {
			if string(rBodyBytes) == "fkzXYeGRjjryynH23upDQK3584vG8SmE" {
				sess.PushMsg("&#9989; ... using InClusterConfig")
				cfg, err = rest.InClusterConfig()
			}
		}
		if cfg == nil {
			sess.PushMsg("ERROR" + err.Error())
			panic(err.Error())
		}
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

		// //		<dryRun>
		// fmt.Println(k8sChartYaml)
		// return
		// //		</dryRun>
		sess.PushMsg("&#129311; ... k8s.cfg.ServerName: " + cfg.Host)

		// //-----------------------------------test k8s config
		// clientset, err := kubernetes.NewForConfig(cfg)
		// if err != nil {
		// 	panic(err.Error())
		// }
		// nsList, err := clientset.CoreV1().Namespaces().List(context.TODO(), metav1.ListOptions{})
		// if err != nil {
		// 	panic(err.Error())
		// }
		// sess.PushMsg("&#129311;[DEBUG] --- good k8s config because i can list namespaces:")
		// for _, ns := range nsList.Items {
		// 	sess.PushMsg(" ... [DEBUG] --- " + ns.ObjectMeta.Name)
		// }
		// //-------------------------------------

		//basically kubectl apply -f
		sess.PushMsg("&#128640;[DEBUG] --- deployment started")
		err = ssa_k8sChartYaml(turkeyUserId, k8sChartYaml, cfg)
		if err != nil {
			sess.PushMsg("ERROR --- deployment FAILED !!! because" + fmt.Sprint(err))
			panic(err.Error())
		}
		skipadminLink := "https://" + turkeySubdomain + "." + turkeyDomain + "?skipadmin"
		sess.PushMsg("&#128640;[DEBUG] --- deployment completed for: <a href=\"" + skipadminLink + "\" target=\"_blank\"><b>")

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
