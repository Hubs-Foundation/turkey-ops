package internal

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"text/template"
	"time"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"k8s.io/apimachinery/pkg/runtime/serializer/yaml"

	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/discovery"
	"k8s.io/client-go/discovery/cached/memory"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/restmapper"
	"k8s.io/client-go/tools/cache"
)

type K8sSvs struct {
	Cfg       *rest.Config
	ClientSet *kubernetes.Clientset
}

func (k8 K8sSvs) StartWatching_HcNs() (chan struct{}, error) {
	if k8.ClientSet == nil {
		return nil, errors.New("k8.ClientSet == nil")
	}
	watchlist := cache.NewFilteredListWatchFromClient(
		k8.ClientSet.CoreV1().RESTClient(),
		"namespaces",
		"",
		func(options *metav1.ListOptions) {
			options.LabelSelector = "TurkeyId"
		},
	)
	_, controller := cache.NewInformer(
		watchlist,
		&corev1.Namespace{},
		0,
		cache.ResourceEventHandlerFuncs{
			AddFunc: func(obj interface{}) {
				GetLogger().Sugar().Debugf("added: %v", obj)
				ns := obj.(*corev1.Namespace)
				HC_NS_TABLE.Set(ns.Name, HcNsNotes{Labels: ns.Labels, Lastchecked: time.Now()})
			},
			UpdateFunc: func(oldObj, newObj interface{}) {
				GetLogger().Sugar().Debugf("updated: %v", newObj)
			},
			DeleteFunc: func(obj interface{}) {
				GetLogger().Sugar().Debugf("deleted: %v", obj)
			},
		},
	)
	stop := make(chan struct{})
	go controller.Run(stop)
	return stop, nil
}

func NewK8sSvs_local() *K8sSvs {

	cfg, err := rest.InClusterConfig()
	if err != nil {
		GetLogger().Error(err.Error())
		return nil
	}
	clientSet, err := kubernetes.NewForConfig(cfg)
	if err != nil {
		GetLogger().Error(err.Error())
	}
	return &K8sSvs{
		Cfg:       cfg,
		ClientSet: clientSet,
	}
}

var decUnstructured = yaml.NewDecodingSerializer(unstructured.UnstructuredJSONScheme)

func Ssa_k8sChartYaml(ssa_userId, k8sChartYaml string, cfg *rest.Config) error {
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

		// GetLogger().Debug("\n\n\n@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@\n" + k8sYaml + "\n\n\n")
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

		force := true
		// Create or Update the object with SSA // types.ApplyPatchType indicates SSA. // FieldManager specifies the field owner ID.
		_, err = dr.Patch(context.TODO(),
			obj.GetName(), types.ApplyPatchType, data,
			metav1.PatchOptions{
				FieldManager: "ssa_userid-" + ssa_userId,
				Force:        &force,
			})
		if err != nil {
			return err
		}
	}
	return err
}

func K8s_render_yams(yams []string, params interface{}) ([]string, error) {
	var yamls []string
	for _, yam := range yams {
		t, err := template.New("yam").Parse(yam)
		if err != nil {
			return yamls, err
		}
		var buf bytes.Buffer
		t.Execute(&buf, params)
		yaml := buf.String()
		yamls = append(yamls, yaml)
		// if yaml == yam {
		// 	GetLogger().Debug("@@@@@@K8s_render_yams @@@@@@: no change for yam string <" + yam[:32] + "......>")
		// } else {
		// 	GetLogger().Debug("@@@@@@K8s_render_yams @@@@@@ : " + yaml)
		// }
		GetLogger().Debug(fmt.Sprintf("size before: %v, size after: %v ", len(yam), len(yaml)))
	}

	return yamls, nil
}

func K8s_GetAllSecrets(cfg *rest.Config, namespace string) (map[string]map[string][]byte, error) {
	clientset, err := kubernetes.NewForConfig(cfg)
	if err != nil {
		return nil, err
	}
	secretsClient := clientset.CoreV1().Secrets(namespace)
	secrets, err := secretsClient.List(context.Background(), metav1.ListOptions{})
	if err != nil {
		return nil, err
	}
	secretMap := make(map[string]map[string][]byte)
	for _, secret := range secrets.Items {
		secretMap[secret.Name] = secret.Data
	}
	return secretMap, nil
}

func K8s_GetServiceIngress0(cfg *rest.Config, namespace string, serviceName string) (corev1.LoadBalancerIngress, error) {
	clientset, err := kubernetes.NewForConfig(cfg)
	if err != nil {
		return corev1.LoadBalancerIngress{}, err
	}
	svcsClient := clientset.CoreV1().Services(namespace)
	svc, err := svcsClient.Get(context.Background(), serviceName, metav1.GetOptions{})
	if err != nil {
		return corev1.LoadBalancerIngress{}, err
	}
	// GetLogger().Debug(fmt.Sprintf("svc.ObjectMeta: %v", svc.ObjectMeta))
	// GetLogger().Debug(fmt.Sprintf("svc.Status.LoadBalancer: %v", svc.Status.LoadBalancer))
	// GetLogger().Debug(fmt.Sprintf("svc.Status.LoadBalancer.Ingress: %v", svc.Status.LoadBalancer.Ingress))
	// GetLogger().Debug(fmt.Sprintf("svc.Status.LoadBalancer.Ingress[0]: %v", svc.Status.LoadBalancer.Ingress[0]))

	tries := 1
	for len(svc.Status.LoadBalancer.Ingress) < 1 {
		if tries > 10 {
			GetLogger().Warn("got nothing and max retry(10) reached")
			break
		}
		GetLogger().Debug("got nothing -- retrying: " + fmt.Sprint(tries))
		time.Sleep(time.Second * 30)
		svc, _ = svcsClient.Get(context.Background(), serviceName, metav1.GetOptions{})
		tries++
		fmt.Printf("svc: %v\n", svc)
	}
	if len(svc.Status.LoadBalancer.Ingress) < 1 {
		return corev1.LoadBalancerIngress{}, errors.New("retry timeout")
	}

	return svc.Status.LoadBalancer.Ingress[0], nil
}

func K8s_getNs(cfg *rest.Config) (*corev1.NamespaceList, error) {
	clientset, err := kubernetes.NewForConfig(cfg)
	if err != nil {
		return nil, err
	}
	nsList, err := clientset.CoreV1().Namespaces().List(context.Background(), metav1.ListOptions{})
	if err != nil {
		return nil, err
	}
	return nsList, nil

}
