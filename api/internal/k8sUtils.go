package internal

import (
	"bytes"
	"context"
	"encoding/json"
	"strings"
	"text/template"

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

		GetLogger().Debug("\n\n\n@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@\n" + k8sYaml + "\n\n\n")
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
		yamls = append(yamls, buf.String())
	}

	return yamls, nil
}
