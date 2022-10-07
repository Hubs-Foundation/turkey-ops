package handlers

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io/ioutil"
	"main/internal"
	"net/http"
	"time"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/rest"
)

type snapshotCfg struct {
	AccountId    string `json:"account_id"`
	HubId        string `json:"hub_id"`
	SnapshotName string `json:"snapshot_name"`
	DBname       string `json:"dbname"`
}

var HC_snapshot = http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
	if r.URL.Path != "/snapshot" {
		http.Error(w, http.StatusText(http.StatusNotFound), http.StatusNotFound)
		return
	}

	switch r.Method {
	case "POST":
		snapshot_restore(w, r)
	case "GET":
		snapshot_list(w, r)
	case "PUT":
		snapshot_create(w, r)
	}

})

func snapshot_restore(w http.ResponseWriter, r *http.Request) {

}

func snapshot_list(w http.ResponseWriter, r *http.Request) {
	ssCfg, err := validateSnapshotCfg(r)
	if err != nil {
		internal.Logger.Error("bad snapshotCfg: " + err.Error())
		http.Error(w, http.StatusText(http.StatusBadRequest), http.StatusBadRequest)
		return
	}

	//getting k8s config
	internal.Logger.Debug("&#9989; ... using InClusterConfig")
	k8sCfg, err := rest.InClusterConfig()
	// }
	if k8sCfg == nil {
		internal.Logger.Debug("ERROR" + err.Error())
		internal.Logger.Error(err.Error())
		return
	}

	client, err := dynamic.NewForConfig(k8sCfg)
	if err != nil {
		internal.Logger.Error(err.Error())
		return
	}

	volumesnapshotRes := schema.GroupVersionResource{Group: "snapshot.storage.k8s.io", Version: "v1", Resource: "volumesnapshots"}
	ssList, err := client.Resource(volumesnapshotRes).Namespace("hc-"+ssCfg.HubId).List(context.TODO(), metav1.ListOptions{})
	if err != nil {
		internal.Logger.Error(err.Error())
		return
	}
	for _, d := range ssList.Items {
		w.WriteHeader(http.StatusOK)
		w.Header().Set("Content-Type", "application/text")
		w.Write([]byte(fmt.Sprintf("%v", d)))
	}
}

func snapshot_create(w http.ResponseWriter, r *http.Request) {

	ssCfg, err := validateSnapshotCfg(r)
	if err != nil {
		internal.Logger.Error("bad snapshotCfg: " + err.Error())
		http.Error(w, http.StatusText(http.StatusBadRequest), http.StatusBadRequest)
		return
	}

	yamBytes, err := ioutil.ReadFile("./_files/yams/snapshot.yam")
	if err != nil {
		internal.Logger.Error("failed to get ns_hc yam file because: " + err.Error())
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(map[string]interface{}{
			"result": "error @ getting k8s chart file: " + err.Error(),
			"hub_id": ssCfg.HubId,
		})
		return
	}

	renderedYamls, err := internal.K8s_render_yams([]string{string(yamBytes)}, ssCfg)
	if err != nil {
		internal.Logger.Error("failed to render ns_hc yam file because: " + err.Error())
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(map[string]interface{}{
			"result": "error @ rendering k8s chart file: " + err.Error(),
			"hub_id": ssCfg.HubId,
		})
		return
	}

	k8sChartYaml := renderedYamls[0]

	internal.Logger.Debug("&#128640; --- create VolumeSnapshot for: " + ssCfg.HubId)
	err = internal.Ssa_k8sChartYaml(ssCfg.AccountId, k8sChartYaml, internal.Cfg.K8ss_local.Cfg)
	if err != nil {
		internal.Logger.Error("error @ k8s deploy: " + err.Error())
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(map[string]interface{}{
			"result": "error @ k8s deploy: " + err.Error(),
			"hub_id": ssCfg.HubId,
		})
		return
	}

	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(map[string]interface{}{
		"result":        "done",
		"account_id":    ssCfg.AccountId,
		"hub_id":        ssCfg.HubId,
		"snapshot_name": ssCfg.SnapshotName,
	})

}

func validateSnapshotCfg(r *http.Request) (snapshotCfg, error) {
	var cfg snapshotCfg

	rBodyBytes, err := ioutil.ReadAll(r.Body)
	if err != nil {
		internal.Logger.Error("ERROR @ reading r.body, error = " + err.Error())
		return cfg, err
	}

	err = json.Unmarshal(rBodyBytes, &cfg)
	if err != nil {
		internal.Logger.Error("bad hcCfg: " + string(rBodyBytes))
		return cfg, err
	}

	// AccountId
	if cfg.AccountId == "" {
		internal.Logger.Debug("AccountId unspecified, using: " + cfg.AccountId)
		return cfg, errors.New("bad input, missing AccountId")
	}
	// HubId
	if cfg.HubId == "" {
		internal.Logger.Debug("HubId unspecified, using: " + cfg.HubId)
		return cfg, errors.New("bad input, missing HubId")
	}

	ss_suffix := time.Now().Format("20060102150405")
	cfg.SnapshotName = "snapshot-" + ss_suffix
	cfg.DBname = "ret_" + cfg.HubId

	return cfg, nil
}
