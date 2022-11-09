package handlers

import (
	"context"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"main/internal"
	"net/http"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
)

type snapshotCfg struct {
	AccountId    string `json:"account_id"`
	HubId        string `json:"hub_id"`
	SnapshotName string `json:"snapshot_name"`
	DBname       string `json:"dbname"`
	BucketName   string `json:"bucket_name"`
	RestoreSize  string `json:"restore_size"`
}

const BACKUPBUCKET = "turkeyfs"

var HC_snapshot = http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
	if r.URL.Path != "/snapshot" {
		http.Error(w, http.StatusText(http.StatusNotFound), http.StatusNotFound)
		return
	}

	ssCfg, err := makeSnapshotCfg(r)
	if err != nil {
		internal.Logger.Error("bad snapshotCfg: " + err.Error())
		http.Error(w, http.StatusText(http.StatusBadRequest), http.StatusBadRequest)
		return
	}

	switch r.Method {
	// case "POST":
	// 	snapshot_restore(w, r)
	case "GET":
		snapshot_list(w, r, ssCfg)
	case "PUT":
		snapshot_create(w, r, ssCfg)
	case "DELETE":
		snapshot_delete(w, r, ssCfg)
	}
})

func snapshot_delete(w http.ResponseWriter, r *http.Request, ssCfg snapshotCfg) {
	client, err := dynamicClient()
	if err != nil {
		return
	}

	volumesnapshotRes := schema.GroupVersionResource{Group: "snapshot.storage.k8s.io", Version: "v1", Resource: "volumesnapshots"}
	err = client.Resource(volumesnapshotRes).Namespace("hc-"+ssCfg.HubId).Delete(context.TODO(), ssCfg.SnapshotName, metav1.DeleteOptions{})
	if err != nil {
		internal.Logger.Error("failed to delete snapshot: " + err.Error())
		http.Error(w, http.StatusText(http.StatusBadRequest), http.StatusBadRequest)
		return
	}

	err = deleteSqlDump(ssCfg.HubId, ssCfg.SnapshotName, ssCfg.BucketName)
	if err != nil {
		internal.Logger.Error("failed to delete sqldump: " + err.Error())
		http.Error(w, http.StatusText(http.StatusBadRequest), http.StatusBadRequest)
		return
	}

	w.WriteHeader(http.StatusOK)
}

// func snapshot_restore(w http.ResponseWriter, r *http.Request) {
// 	ssCfg, err := makeSnapshotCfg(r)
// 	if err != nil {
// 		internal.Logger.Error("bad snapshotCfg: " + err.Error())
// 		http.Error(w, http.StatusText(http.StatusBadRequest), http.StatusBadRequest)
// 		return
// 	}

// 	//getting k8s config
// 	internal.Logger.Debug("&#9989; ... using InClusterConfig")
// 	k8sCfg, err := rest.InClusterConfig()
// 	// }
// 	if k8sCfg == nil {
// 		internal.Logger.Debug("ERROR" + err.Error())
// 		internal.Logger.Error(err.Error())
// 		return
// 	}

// 	client, err := dynamic.NewForConfig(k8sCfg)
// 	if err != nil {
// 		internal.Logger.Error(err.Error())
// 		return
// 	}

// 	volumesnapshotRes := schema.GroupVersionResource{Group: "snapshot.storage.k8s.io", Version: "v1", Resource: "volumesnapshots"}
// 	obj, err := client.Resource(volumesnapshotRes).Namespace("hc-"+ssCfg.HubId).Get(context.TODO(), ssCfg.SnapshotName, metav1.GetOptions{})
// 	if err != nil {
// 		internal.Logger.Error("unable to get the volsnapshot object: " + err.Error())
// 		http.Error(w, http.StatusText(http.StatusBadRequest), http.StatusBadRequest)
// 		return
// 	}

// 	var vs volsnapshotv1.VolumeSnapshot
// 	err = runtime.DefaultUnstructuredConverter.FromUnstructured(obj.UnstructuredContent(), &vs)
// 	if err != nil {
// 		internal.Logger.Error("unable to convert the unstrucctured object to volumesnapshot object: " + err.Error())
// 		http.Error(w, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
// 		return
// 	}

// 	if vs.Status.RestoreSize.String() == "" {
// 		internal.Logger.Error(fmt.Sprintf("unable to get the volsnapshot %s restore size", ssCfg.SnapshotName))
// 		http.Error(w, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
// 		return
// 	}

// 	ssCfg.RestoreSize = vs.Status.RestoreSize.String()

// 	// create pvc from the volumesnapshot
// 	yamBytes, err := ioutil.ReadFile("./_files/yams/restore.yam")
// 	if err != nil {
// 		internal.Logger.Error("failed to get restore yam file because: " + err.Error())
// 		w.WriteHeader(http.StatusInternalServerError)
// 		json.NewEncoder(w).Encode(map[string]interface{}{
// 			"result": "error @ getting k8s chart file: " + err.Error(),
// 			"hub_id": ssCfg.HubId,
// 		})
// 		return
// 	}

// 	renderedYamls, err := internal.K8s_render_yams([]string{string(yamBytes)}, ssCfg)
// 	if err != nil {
// 		internal.Logger.Error("failed to render restore yam file because: " + err.Error())
// 		w.WriteHeader(http.StatusInternalServerError)
// 		json.NewEncoder(w).Encode(map[string]interface{}{
// 			"result": "error @ rendering k8s chart file: " + err.Error(),
// 			"hub_id": ssCfg.HubId,
// 		})
// 		return
// 	}

// 	k8sChartYaml := renderedYamls[0]

// 	internal.Logger.Debug("&#128640; --- create pvc from volumesnapshot for: " + ssCfg.HubId)
// 	err = internal.Ssa_k8sChartYaml(ssCfg.AccountId, k8sChartYaml, internal.Cfg.K8ss_local.Cfg)
// 	if err != nil {
// 		internal.Logger.Error("error @ k8s deploy: " + err.Error())
// 		w.WriteHeader(http.StatusInternalServerError)
// 		json.NewEncoder(w).Encode(map[string]interface{}{
// 			"result": "error @ k8s deploy: " + err.Error(),
// 			"hub_id": ssCfg.HubId,
// 		})
// 		return
// 	}

// 	// scale delpoyment down to 0 before the backup
// 	err = hc_switch(ssCfg.HubId, "down")
// 	if err != nil {
// 		w.WriteHeader(http.StatusInternalServerError)
// 		json.NewEncoder(w).Encode(map[string]interface{}{
// 			"error":  err.Error(),
// 			"hub_id": ssCfg.HubId,
// 		})
// 		return
// 	}

// 	// w.WriteHeader(http.StatusOK)
// 	// json.NewEncoder(w).Encode(map[string]interface{}{
// 	// 	"result":            "done",
// 	// 	"account_id":        ssCfg.AccountId,
// 	// 	"hub_id":            ssCfg.HubId,
// 	// 	"snapshot_name":     ssCfg.SnapshotName,
// 	// 	"snapshot_size_raw": vs.Status.RestoreSize.String(),
// 	// })

// }

func snapshot_list(w http.ResponseWriter, r *http.Request, ssCfg snapshotCfg) {
	client, err := dynamicClient()
	if err != nil {
		return
	}

	volumesnapshotRes := schema.GroupVersionResource{Group: "snapshot.storage.k8s.io", Version: "v1", Resource: "volumesnapshots"}
	ssList, err := client.Resource(volumesnapshotRes).Namespace("hc-"+ssCfg.HubId).List(context.TODO(), metav1.ListOptions{})
	if err != nil {
		internal.Logger.Error(err.Error())
		return
	}

	var snList []string
	for _, d := range ssList.Items {
		snList = append(snList, d.GetName())
	}

	w.WriteHeader(http.StatusOK)
	w.Header().Set("Content-Type", "application/text")
	w.Write([]byte(fmt.Sprintf("%v", snList)))
}

func snapshot_create(w http.ResponseWriter, r *http.Request, ssCfg snapshotCfg) {
	// scale delpoyment down to 0 before the backup
	err := hc_switch(ssCfg.HubId, "down")
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(map[string]interface{}{
			"error":  err.Error(),
			"hub_id": ssCfg.HubId,
		})
		return
	}
	// create the volumentsnapshot
	yamBytes, err := ioutil.ReadFile("./_files/yams/addons/snapshot.yam")
	if err != nil {
		internal.Logger.Error("failed to get snapshot yam file because: " + err.Error())
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(map[string]interface{}{
			"result": "error @ getting k8s chart file: " + err.Error(),
			"hub_id": ssCfg.HubId,
		})
		return
	}

	renderedYamls, err := internal.K8s_render_yams([]string{string(yamBytes)}, ssCfg)
	if err != nil {
		internal.Logger.Error("failed to render snapshot yam file because: " + err.Error())
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

	// create the db dump
	instanceId, err := getInstanceId()
	if err != nil {
		internal.Logger.Error("failed to get DB instanceId: " + err.Error())
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(map[string]interface{}{
			"result": "error @ gettting DB instanceId: " + err.Error(),
			"hub_id": ssCfg.HubId,
		})
		return
	}
	if instanceId == "" {
		internal.Logger.Error("DB instanceId not found")
		w.WriteHeader(http.StatusNotFound)
		json.NewEncoder(w).Encode(map[string]interface{}{
			"result": "error @ DB instanceId not found: ",
			"hub_id": ssCfg.HubId,
		})
		return
	}

	exportStatus, err := createSqlDump(ssCfg.HubId, ssCfg.SnapshotName, ssCfg.BucketName, ssCfg.DBname)

	if err != nil {
		internal.Logger.Error("create DB dump error: " + err.Error())
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(map[string]interface{}{
			"result": "error @ creating DB dump: " + err.Error(),
			"hub_id": ssCfg.HubId,
		})
		return
	}

	internal.Logger.Info("Instance export reponse status: " + exportStatus)
	// scale up the deloyment after the backup
	err = hc_switch(ssCfg.HubId, "up")
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(map[string]interface{}{
			"error":  err.Error(),
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
