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

	"github.com/google/uuid"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/utils/pointer"

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

	coordinationv1 "k8s.io/api/coordination/v1"
	k8errors "k8s.io/apimachinery/pkg/api/errors"
	coordinationclientv1 "k8s.io/client-go/kubernetes/typed/coordination/v1"
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
				ns := newObj.(*corev1.Namespace)
				HC_NS_TABLE.Set(ns.Name, HcNsNotes{Labels: ns.Labels, Lastchecked: time.Now()})
			},
			DeleteFunc: func(obj interface{}) {
				GetLogger().Sugar().Debugf("deleted: %v", obj)
				ns := obj.(*corev1.Namespace)
				HC_NS_TABLE.Del(ns.Name)
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
		ssaResult, err := dr.Patch(context.TODO(),
			obj.GetName(), types.ApplyPatchType, data,
			metav1.PatchOptions{
				FieldManager: "ssa_userid-" + ssa_userId,
				Force:        &force,
			})
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

// ########################## k8Locker ##########################

type k8Locker struct {
	leaseClient coordinationclientv1.LeaseInterface
	namespace   string
	name        string
	clientID    string
	retryWait   time.Duration
	maxWait     time.Duration
	ttl         time.Duration
}

// NewLocker creates a Locker
func NewK8Locker(k8Cfg *rest.Config, namespace string) (*k8Locker, error) {
	name := "turkey-ops"

	// create the Lease if it doesn't exist
	leaseClient := Cfg.K8ss_local.ClientSet.CoordinationV1().Leases(namespace)
	_, err := leaseClient.Get(context.TODO(), name, metav1.GetOptions{})
	if err != nil {
		if !k8errors.IsNotFound(err) {
			return nil, err
		}
		lease := &coordinationv1.Lease{
			ObjectMeta: metav1.ObjectMeta{
				Name: name,
			},
			Spec: coordinationv1.LeaseSpec{
				LeaseTransitions: pointer.Int32Ptr(0),
			},
		}
		_, err := leaseClient.Create(context.TODO(), lease, metav1.CreateOptions{})
		if err != nil {
			return nil, err
		}
	}
	return &k8Locker{
		name:        name,
		namespace:   namespace,
		clientID:    uuid.New().String(),
		retryWait:   500 * time.Millisecond,
		maxWait:     30 * time.Second,
		leaseClient: leaseClient,
	}, nil
}

// Lock will block until the client is the holder of the Lease resource
func (l *k8Locker) Lock() {
	ttl := l.maxWait

	// block until we get a lock
	for {
		if ttl < 0 {
			panic(fmt.Sprintf("timeout while trying to get a lease for lock: %v", l))
		}
		// get the Lease
		lease, err := l.leaseClient.Get(context.TODO(), l.name, metav1.GetOptions{})
		if err != nil {
			panic(fmt.Sprintf("could not get Lease resource for lock: %v", err))
		}

		if lease.Spec.HolderIdentity != nil {
			if lease.Spec.LeaseDurationSeconds == nil {
				// The lock is already held and has no expiry
				time.Sleep(l.retryWait)
				ttl -= l.retryWait
				continue
			}

			acquireTime := lease.Spec.AcquireTime.Time
			leaseDuration := time.Duration(*lease.Spec.LeaseDurationSeconds) * time.Second

			if acquireTime.Add(leaseDuration).After(time.Now()) {
				// The lock is already held and hasn't expired yet
				time.Sleep(l.retryWait)
				ttl -= l.retryWait
				continue
			}
		}

		// nobody holds the lock, try and lock it
		lease.Spec.HolderIdentity = pointer.StringPtr(l.clientID)
		if lease.Spec.LeaseTransitions != nil {
			lease.Spec.LeaseTransitions = pointer.Int32Ptr((*lease.Spec.LeaseTransitions) + 1)
		} else {
			lease.Spec.LeaseTransitions = pointer.Int32Ptr((*lease.Spec.LeaseTransitions) + 1)
		}
		lease.Spec.AcquireTime = &metav1.MicroTime{time.Now()}
		if l.ttl.Seconds() > 0 {
			lease.Spec.LeaseDurationSeconds = pointer.Int32Ptr(int32(l.ttl.Seconds()))
		}
		_, err = l.leaseClient.Update(context.TODO(), lease, metav1.UpdateOptions{})
		if err == nil {
			// we got the lock, break the loop
			break
		}

		if !k8errors.IsConflict(err) {
			// if the error isn't a conflict then something went horribly wrong
			panic(fmt.Sprintf("lock: error when trying to update Lease: %v", err))
		}

		// Another client beat us to the lock
		time.Sleep(l.retryWait)
		ttl -= l.retryWait
	}
}

// Unlock will remove the client as the holder of the Lease resource
func (l *k8Locker) Unlock() {

	lease, err := l.leaseClient.Get(context.TODO(), l.name, metav1.GetOptions{})
	if err != nil {
		panic(fmt.Sprintf("could not get Lease resource for lock: %v", err))
	}

	// the holder has to have a value and has to be our ID for us to be able to unlock
	if lease.Spec.HolderIdentity == nil {
		panic("unlock: no lock holder value")
	}

	if *lease.Spec.HolderIdentity != l.clientID {
		panic("unlock: not the lock holder")
	}

	lease.Spec.HolderIdentity = nil
	lease.Spec.AcquireTime = nil
	lease.Spec.LeaseDurationSeconds = nil
	_, err = l.leaseClient.Update(context.TODO(), lease, metav1.UpdateOptions{})
	if err != nil {
		panic(fmt.Sprintf("unlock: error when trying to update Lease: %v", err))
	}
}

// func (k8 K8sSvs) Init_LeaseBasedLock(namespaceName, lockName string) (*apiCoordV1.Lease, error) {
// 		//try 3 times to create one
// 		tries := 3
// 		for tries > 0 {
// 			if !k8sErrors.IsNotFound(err) {
// 				tries--
// 				time.Sleep(100 * time.Millisecond)
// 				continue
// 			}
// 			lease, err = k8.ClientSet.CoordinationV1().Leases(namespaceName).Create(context.TODO(), &apiCoordV1.Lease{
// 				ObjectMeta: metav1.ObjectMeta{Name: lockName}, Spec: apiCoordV1.LeaseSpec{LeaseTransitions: pointer.Int32Ptr(0)}}, metav1.CreateOptions{})
// 			if err != nil {
// 				tries--
// 				time.Sleep(100 * time.Millisecond)
// 				return nil, err
// 			}

// 		}
// }

// func (k8 K8sSvs) Acquire_LeaseBasedLock(namespaceName, lockName string) (*apiCoordV1.Lease, error) {
// 	uuid := uuid.New().String()

// 	//get lease
// 	lease, err := k8.ClientSet.CoordinationV1().Leases(namespaceName).Get(context.TODO(), lockName, metav1.GetOptions{})
// 	if err != nil {

// 	}
// 	ns, err := k8.ClientSet.CoreV1().Namespaces().Get(context.Background(), namespaceName, metav1.GetOptions{})

// 	return uuid, nil
// }

// func (k8 K8sSvs) Release_LeaseBasedLock(namespaceName, labelKey, uuid string) error {

// 	ns, err := k8.ClientSet.CoreV1().Namespaces().Get(context.Background(), namespaceName, metav1.GetOptions{})
// 	if err != nil {
// 		return err
// 	}
// 	if ns.Labels[labelKey] != uuid {
// 		return errors.New("unexpected uuid " + ns.Labels[labelKey])
// 	} else {
// 		ns.Labels[labelKey] = ""
// 	}
// 	_, err = k8.ClientSet.CoreV1().Namespaces().Update(context.Background(), ns, metav1.UpdateOptions{})
// 	if err != nil {
// 		return err
// 	}

// 	ns_updated, err := k8.ClientSet.CoreV1().Namespaces().Get(context.Background(), namespaceName, metav1.GetOptions{})
// 	if err != nil {
// 		return err
// 	}
// 	if ns_updated.Labels[labelKey] != "" {
// 		return errors.New("lock verfication failed: expecting: " + uuid + ", getting: " + ns_updated.Labels[labelKey])
// 	}

// 	return nil
// }
