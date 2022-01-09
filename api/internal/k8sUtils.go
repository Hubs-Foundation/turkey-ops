package internal

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"text/template"
	"time"

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
)

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
		if yaml == yam {
			GetLogger().Debug("@@@@@@K8s_render_yams @@@@@@: no change for yam string <" + yam[:32] + "......>")
		} else {
			GetLogger().Debug("@@@@@@K8s_render_yams @@@@@@ : " + yaml)
		}
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

func K8s_GetServiceExtIp(cfg *rest.Config, namespace string, serviceName string) (string, error) {
	clientset, err := kubernetes.NewForConfig(cfg)
	if err != nil {
		return "", err
	}
	svcsClient := clientset.CoreV1().Services(namespace)
	svc, err := svcsClient.Get(context.Background(), serviceName, metav1.GetOptions{})
	if err != nil {
		return "", err
	}

	tries := 1
	for len(svc.Spec.ExternalIPs) < 1 {
		if tries > 20 {
			GetLogger().Warn("got nothing and max retry(20) reached")
			break
		}
		GetLogger().Debug("got nothing -- retrying: " + fmt.Sprint(tries))
		time.Sleep(time.Second * 15)
		svc, _ = svcsClient.Get(context.Background(), serviceName, metav1.GetOptions{})
		tries++
	}
	if len(svc.Spec.ExternalIPs) < 1 {
		return "", nil
	}

	return svc.Spec.ExternalIPs[0], nil
}
