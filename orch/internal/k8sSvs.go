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
	networkingv1 "k8s.io/api/networking/v1"
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

	k8errors "k8s.io/apimachinery/pkg/api/errors"
)

type K8sSvs struct {
	Cfg       *rest.Config
	ClientSet *kubernetes.Clientset
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

func (k8 K8sSvs) StartWatching_HcNs() (chan struct{}, error) {
	if k8.ClientSet == nil {
		return nil, errors.New("k8.ClientSet == nil")
	}
	watchlist := cache.NewFilteredListWatchFromClient(
		k8.ClientSet.CoreV1().RESTClient(),
		"namespaces",
		"",
		func(options *metav1.ListOptions) {
			options.LabelSelector = "hub_id,subdomain"
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
				HC_NS_MAN.Set(ns.Name, HcNsNotes{Labels: ns.Labels, Lastchecked: time.Now()})
			},
			UpdateFunc: func(oldObj, newObj interface{}) {
				GetLogger().Sugar().Debugf("updated: %v", newObj)
				ns := newObj.(*corev1.Namespace)
				if ns.Annotations["deleting"] == "true" {
					HC_NS_MAN.Del(ns.Name)
					return
				}
				HC_NS_MAN.Set(ns.Name, HcNsNotes{Labels: ns.Labels, Lastchecked: time.Now()})
			},
			DeleteFunc: func(obj interface{}) {
				GetLogger().Sugar().Debugf("deleted: %v", obj)
				ns := obj.(*corev1.Namespace)
				HC_NS_MAN.Del(ns.Name)
			},
		},
	)
	stop := make(chan struct{})
	go controller.Run(stop)
	return stop, nil
}

// wait until len(Pods.Items) drops to or below targetCnt, only cares about running pods
func (k8 K8sSvs) WaitForPodKill(namespace string, timeout time.Duration, targetCnt int) error {
	if k8.ClientSet == nil {
		return errors.New("k8.ClientSet == nil")
	}
	wait := 5 * time.Second
	podCount := int(^uint(0) >> 1)
	for podCount > targetCnt && timeout > 0 {
		time.Sleep(wait)
		timeout -= wait
		pods, err := k8.ClientSet.CoreV1().Pods(namespace).List(context.Background(), metav1.ListOptions{FieldSelector: "status.phase=Running"})
		if err != nil {
			return err
		}
		podCount = len(pods.Items)
		Logger.Sugar().Infof("[%v] %v -> %v", namespace, podCount, targetCnt)
	}
	if timeout <= 0 {
		return errors.New("timeout")
	}
	pods, err := k8.ClientSet.CoreV1().Pods(namespace).List(context.Background(), metav1.ListOptions{})
	if err != nil {
		return err
	}
	Logger.Sugar().Infof("[%v] exit cnt: %v", namespace, len(pods.Items))
	return nil
}

func (k8 K8sSvs) PatchNsAnnotation(namespace string, AnnotationKey, AnnotationValue string) error {
	if k8.ClientSet == nil {
		return errors.New("k8.ClientSet == nil")
	}
	ns, err := k8.ClientSet.CoreV1().Namespaces().Get(context.Background(), namespace, metav1.GetOptions{})
	if err != nil {
		return err
	}
	ns.Annotations[AnnotationKey] = AnnotationValue
	_, err = k8.ClientSet.CoreV1().Namespaces().Update(context.Background(), ns, metav1.UpdateOptions{})
	if err != nil {
		return err
	}
	return nil
}

func (k8 K8sSvs) GetFromHubNsLabel(hubId string, label string) (string, error) {
	ns, err := k8.ClientSet.CoreV1().Namespaces().Get(context.Background(), "hc-"+hubId, metav1.GetOptions{})
	if err != nil {
		return "", err
	}
	return ns.Labels[label], nil
}
func (k8 K8sSvs) GetFromHubsItaLabel(hubId string, label string) (string, error) {
	d, err := k8.ClientSet.AppsV1().Deployments("hc-"+hubId).Get(context.Background(), "ita", metav1.GetOptions{})
	if err != nil {
		return "", err
	}
	return d.Labels[label], nil
}

func (k8 K8sSvs) GetOrCreateTrcIngress() (*networkingv1.Ingress, error) {

	namespace := Cfg.PodNS
	ingressName := "turkey-return-center"

	if k8.ClientSet == nil {
		return nil, errors.New("k8.ClientSet == nil")
	}
	ig, err := k8.ClientSet.NetworkingV1().Ingresses(namespace).Get(context.Background(), ingressName, metav1.GetOptions{})
	if err == nil {
		return ig, nil
	}
	if k8errors.IsNotFound(err) {
		pathType := networkingv1.PathTypeExact
		ig, err = k8.ClientSet.NetworkingV1().Ingresses(namespace).Create(context.Background(),
			&networkingv1.Ingress{
				ObjectMeta: metav1.ObjectMeta{
					Name: ingressName,
					Annotations: map[string]string{
						`haproxy.org/request-set-header`: `trc .`,
						`kubernetes.io/ingress.class`:    `haproxy`,
					},
				},
				Spec: networkingv1.IngressSpec{
					Rules: []networkingv1.IngressRule{
						{
							Host: "orch." + Cfg.Domain,
							IngressRuleValue: networkingv1.IngressRuleValue{
								HTTP: &networkingv1.HTTPIngressRuleValue{
									Paths: []networkingv1.HTTPIngressPath{
										{
											Path:     "/trc",
											PathType: &pathType,
											Backend: networkingv1.IngressBackend{
												Service: &networkingv1.IngressServiceBackend{
													Name: "turkeyorch",
													Port: networkingv1.ServiceBackendPort{
														Number: 888,
													}}}},
									}}},
						},
					},
				},
			},
			metav1.CreateOptions{},
		)
	}
	return ig, err
}

func (k8 K8sSvs) TrcIg_deleteHost(host string) error {
	Logger.Debug("TrcIg_deleteHost, host: " + host)
	RetryFunc(
		600*time.Second, 3*time.Second,
		func() error {
			trcIg, err := k8.GetOrCreateTrcIngress()
			if err != nil {
				return err
			}
			for idx, igRule := range trcIg.Spec.Rules {
				if igRule.Host == host {
					trcIg.Spec.Rules = append(trcIg.Spec.Rules[:idx], trcIg.Spec.Rules[idx+1:]...)
					break
				}
			}
			_, err = k8.ClientSet.NetworkingV1().Ingresses(Cfg.PodNS).Update(context.Background(),
				trcIg, metav1.UpdateOptions{})
			return err
		})
	return nil
}

func (k8 K8sSvs) GetOrCreateTrcConfigmap() (*corev1.ConfigMap, error) {

	namespace := Cfg.PodNS
	cmName := "turkey-return-center"

	if k8.ClientSet == nil {
		return nil, errors.New("k8.ClientSet == nil")
	}
	cm, err := k8.ClientSet.CoreV1().ConfigMaps(namespace).Get(context.Background(), cmName, metav1.GetOptions{})
	if k8errors.IsNotFound(err) {
		cm, err = k8.ClientSet.CoreV1().ConfigMaps(namespace).Create(context.Background(),
			&corev1.ConfigMap{
				ObjectMeta: metav1.ObjectMeta{
					Name: cmName,
				},
				Data: map[string]string{"-": "-"},
			},
			metav1.CreateOptions{})
	}

	return cm, err

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
		ssaResult, err := dr.Patch(context.TODO(),
			obj.GetName(), types.ApplyPatchType, data,
			metav1.PatchOptions{
				FieldManager: "ssa_userid-" + ssa_userId,
				Force:        &force,
			})
		if err != nil {
			return err
		}
		// Logger.Sugar().Debugf("ssaResult: %v", ssaResult.Object)
		jsonBytes, err := json.Marshal(ssaResult.Object)
		if err != nil {
			Logger.Sugar().Debugf("err=%v", err)
		}
		Logger.Debug("ssa-result: " + string(jsonBytes))

		// Logger.Sugar().Debugf("ssaResult: %v", func() string { jsonBytes, _ := json.Marshal(ssaResult.Object); return string(jsonBytes) })

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

	tries := 15
	for len(svc.Status.LoadBalancer.Ingress) < 1 {
		if tries < 1 {
			GetLogger().Warn("timeout")
			return corev1.LoadBalancerIngress{}, errors.New("retry timeout")
		}
		GetLogger().Info("nothing -- retrying: " + fmt.Sprint(tries))
		time.Sleep(time.Second * 60)
		svc, _ = svcsClient.Get(context.Background(), serviceName, metav1.GetOptions{})
		tries--
		fmt.Printf("svc: %v\n", svc)
	}

	return svc.Status.LoadBalancer.Ingress[0], nil
}

func K8s_GetIngressIngress0(cfg *rest.Config, namespace string, ingressName string) (corev1.LoadBalancerIngress, error) {
	clientset, err := kubernetes.NewForConfig(cfg)
	if err != nil {
		return corev1.LoadBalancerIngress{}, err
	}
	igsClient := clientset.NetworkingV1().Ingresses(namespace)
	ig, err := igsClient.Get(context.Background(), ingressName, metav1.GetOptions{})
	if err != nil {
		return corev1.LoadBalancerIngress{}, err
	}

	tries := 15
	for len(ig.Status.LoadBalancer.Ingress) < 1 {
		if tries < 1 {
			GetLogger().Warn("timeout")
			return corev1.LoadBalancerIngress{}, errors.New("retry timeout")
		}
		GetLogger().Info("nothing -- retrying: " + fmt.Sprint(tries))
		time.Sleep(time.Second * 60)
		ig, _ = igsClient.Get(context.Background(), ingressName, metav1.GetOptions{})
		tries--
		fmt.Printf("ig: %v\n", ig)
	}

	return ig.Status.LoadBalancer.Ingress[0], nil
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
