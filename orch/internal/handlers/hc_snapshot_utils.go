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

	sqladmin "google.golang.org/api/sqladmin/v1beta4"
	"google.golang.org/api/storage/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
)

// create a dynamic kuberntes client
func dynamicClient() (dynamic.Interface, error) {
	//getting k8s config
	internal.Logger.Debug("&#9989; ... using InClusterConfig")
	k8sCfg, err := rest.InClusterConfig()
	// }
	if k8sCfg == nil {
		internal.Logger.Debug("ERROR" + err.Error())
		internal.Logger.Error(err.Error())
		return nil, err
	}

	client, err := dynamic.NewForConfig(k8sCfg)
	if err != nil {
		internal.Logger.Error(err.Error())
		return nil, err
	}

	return client, nil
}

// create a dynamic kuberntes client
func kubernetesClient() (*kubernetes.Clientset, error) {
	//getting k8s config
	internal.Logger.Debug("&#9989; ... using InClusterConfig")
	k8sCfg, err := rest.InClusterConfig()
	// }
	if k8sCfg == nil {
		internal.Logger.Debug("ERROR" + err.Error())
		internal.Logger.Error(err.Error())
		return nil, err
	}

	client, err := kubernetes.NewForConfig(k8sCfg)
	if err != nil {
		internal.Logger.Debug("ERROR" + err.Error())
		internal.Logger.Error(err.Error())
		return nil, err
	}

	return client, nil
}

// validate and create snapshotConfig
func makeSnapshotCfg(r *http.Request) (snapshotCfg, error) {
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
	// SnapshotName
	if cfg.SnapshotName == "" {
		ss_suffix := time.Now().Format("20060102150405")
		cfg.SnapshotName = "snapshot-" + ss_suffix
	}
	// BucketName
	if cfg.BucketName == "" {
		cfg.BucketName = BACKUPBUCKET
	}
	// DdatabaseName
	cfg.DBname = "ret_" + cfg.HubId

	return cfg, nil
}

// get db instance id
func getInstanceId() (string, error) {
	//getting k8s config
	internal.Logger.Debug("&#9989; ... using InClusterConfig")
	k8sCfg, err := rest.InClusterConfig()
	// }
	if k8sCfg == nil || err != nil {
		internal.Logger.Debug("ERROR" + err.Error())
		internal.Logger.Error(err.Error())
		return "", err
	}

	internal.Logger.Debug("&#129311; k8s.k8sCfg.Host == " + k8sCfg.Host)
	clientset, err := kubernetes.NewForConfig(k8sCfg)
	if err != nil {
		internal.Logger.Error(err.Error())
		return "", err
	}

	nodes, err := clientset.CoreV1().Nodes().List(context.Background(), metav1.ListOptions{})
	if err != nil {
		internal.Logger.Error(err.Error())
		return "", err
	}

	for _, n := range nodes.Items {
		if n.Labels["stackname"] != "" {
			return n.Labels["stackname"], nil
		}
	}

	return "", nil
}

// create sql dump
func createSqlDump(hubsId, snapshotName, bucketName, databaseName string) (string, error) {
	projectId := internal.Cfg.Gcps.ProjectId

	instanceId, err := getInstanceId()
	if err != nil {
		return "", err
	}
	if instanceId == "" {
		return "", errors.New("instanceId not found")
	}

	ctx := context.Background()
	sqladminService, err := sqladmin.NewService(ctx)
	if err != nil {
		return "", err
	}

	instanceExportRequest := sqladmin.InstancesExportRequest{
		ExportContext: &sqladmin.ExportContext{
			Databases: []string{databaseName},
			FileType:  "SQL",
			Uri:       fmt.Sprintf("gs://%s/hc-%s/%s.gz", bucketName, hubsId, snapshotName),
		},
	}

	call, err := sqladminService.Instances.Export(projectId, instanceId, &instanceExportRequest).Do()
	if err != nil {
		return "", err
	}

	return call.Status, nil

}

// delete sql dump
func deleteSqlDump(hubsId, snapshotName, bucketName string) error {
	ctx := context.Background()
	storageService, err := storage.NewService(ctx)
	if err != nil {
		return err
	}

	err = storageService.Objects.Delete(bucketName, fmt.Sprintf("hc-%s/%s.gz", hubsId, snapshotName)).Do()
	if err != nil {
		return err
	}
	return nil
}

// import sql dump
func importSqlDump(hubsId, snapshotName, bucketName, databaseName string) (string, error) {
	projectId := internal.Cfg.Gcps.ProjectId

	instanceId, err := getInstanceId()
	if err != nil {
		return "", err
	}
	if instanceId == "" {
		return "", errors.New("instanceId not found")
	}
	ctx := context.Background()
	sqladminService, err := sqladmin.NewService(ctx)
	if err != nil {
		return "", err
	}

	instanceImportRequest := sqladmin.InstancesImportRequest{
		ImportContext: &sqladmin.ImportContext{
			Database: databaseName,
			FileType: "SQL",
			Uri:      fmt.Sprintf("gs://%s/hc-%s/%s.gz", bucketName, hubsId, snapshotName),
		},
	}

	call, err := sqladminService.Instances.Import(projectId, instanceId, &instanceImportRequest).Do()
	if err != nil {
		return "", err
	}

	return call.Status, nil

}
