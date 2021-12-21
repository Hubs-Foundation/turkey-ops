package internal

import (
	"context"
	"encoding/json"
	"strings"

	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/serializer/yaml"

	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/discovery"
	"k8s.io/client-go/discovery/cached/memory"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/restmapper"
)

var decUnstructured = yaml.NewDecodingSerializer(unstructured.UnstructuredJSONScheme)

func Ssa_k8sChartYaml(userId, k8sChartYaml string, cfg *rest.Config) error {
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
		// fmt.Println("\n@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@\n\n\n")
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

		force := true
		// Create or Update the object with SSA // types.ApplyPatchType indicates SSA. // FieldManager specifies the field owner ID.
		_, err = dr.Patch(context.TODO(),
			obj.GetName(), types.ApplyPatchType, data,
			metav1.PatchOptions{
				FieldManager: "turkey-userid-" + userId,
				Force:        &force,
			})
		if err != nil {
			return err
		}
	}
	return err
}
