package internal

import (
	"errors"
	"fmt"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/tanfarming/goutils/pkg/kubelocker"
	corev1 "k8s.io/api/core/v1"
)

// curl -X PATCH ita:6000/deploy?app=hubs?file=<name-of-the-file-under-/storage/ita-uploads>
var Deploy = http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
	features := cfg.Features.Get()
	if !features.customClient ||
		(r.URL.Path != "/deploy" && r.URL.Path != "/api/ita/deploy") ||
		(r.Method != "PATCH" && r.Method != "POST") {
		http.Error(w, http.StatusText(http.StatusNotFound), http.StatusNotFound)
		return
	}

	if cfg.CustomDomain == "" {
		http.Error(w, "custom domain is required but not configured", http.StatusBadRequest)
	}

	app := r.URL.Query().Get("app")
	if app != "hubs" && app != "spoke" {
		http.Error(w, http.StatusText(http.StatusNotFound), http.StatusNotFound)
		return
	}

	locker, err := kubelocker.Newkubelocker(cfg.K8sClientSet, cfg.PodNS)
	if err != nil {
		Logger.Sugar().Errorf("failed to create locker for namespace: %v", cfg.PodNS)
		return
	}
	err = locker.Lock()
	if err != nil {
		Logger.Sugar().Errorf("failed to lock: err:%v, id: %v, worklog: %v", err, locker.Id(), strings.Join(locker.WorkLog(), ";"))
		return
	}
	Logger.Sugar().Debugf("acquired locker: %v \n", locker.Id())

	defer func() {
		err = locker.Unlock()
		if err != nil {
			Logger.Sugar().Errorf("failed to unlock " + err.Error())
		}
	}()

	switch r.Method {
	case "PATCH":
		if len(r.URL.Query()["file"]) != 1 || r.URL.Query()["file"][0] == "" {
			http.Error(w, "missing: file", http.StatusBadRequest)
			return
		}
		fileName := r.URL.Query()["file"][0]

		err := unzipNdeployCustomClient(app, fileName)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		w.WriteHeader(http.StatusOK)
		fmt.Fprint(w, "deployed: "+fileName)
	case "POST":
		files, err := receiveFileFromReqBody(r)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
		}
		reqId := w.Header().Get("X-Request-Id")

		if len(files) < 1 {
			Logger.Sugar().Debug("didn't receive any file")
			http.Error(w, "got no file, want file", http.StatusInternalServerError)
			return
		}
		err = unzipNdeployCustomClient(app, files[0])
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		w.WriteHeader(http.StatusOK)
		fmt.Fprintf(w, "done, reqId: %v", reqId)
	default:
		w.WriteHeader(http.StatusBadRequest)
		return
	}
	Deployment_setLabel("custom-client", "yes")
	go func() {
		wait := 35 * time.Second
		Logger.Sugar().Debugf("respawning %v pods in %v", app, wait)
		time.Sleep(wait)
		//refresh nfs mount, prevent stale file handle error
		err := killPods("app=" + app)
		if err != nil {
			Logger.Error(err.Error())
		}
	}()
})

var Undeploy = http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
	features := cfg.Features.Get()
	if !features.customClient || (r.URL.Path != "/undeploy" && r.URL.Path != "/api/ita/undeploy") {
		http.Error(w, http.StatusText(http.StatusNotFound), http.StatusNotFound)
		return
	}
	app := r.URL.Query().Get("app")
	if app != "hubs" && app != "spoke" {
		http.Error(w, http.StatusText(http.StatusBadRequest), http.StatusBadRequest)
		return
	}

	err := k8s_removeNfsMount(app)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	Deployment_setLabel("custom-client", "")

	go func() {
		wait := 35 * time.Second
		Logger.Sugar().Debugf("respawning %v pods in %v", app, wait)
		time.Sleep(wait)
		//refresh nfs mount, prevent stale file handle error
		err := killPods("app=" + app)
		if err != nil {
			Logger.Error(err.Error())
		}
	}()

	w.WriteHeader(http.StatusOK)
	fmt.Fprintf(w, "done")
})

////////////////////////////////////////////////////////////////////////////////////////////////////

func unzipNdeployCustomClient(app, fileName string) error {

	dir := "/storage/" + app
	Logger.Sugar().Debugf("unzipNdeployCustomClient, dir: %v, fileName: %v", dir, fileName)

	os.RemoveAll(dir)

	//unzip
	if strings.HasSuffix(fileName, ".tar.gz") {
		err := UnzipTar("/storage/ita_uploads/"+fileName, dir)
		if err != nil {
			return errors.New("failed @ UnzipTar: " + err.Error())
		}
	} else if strings.HasSuffix(fileName, ".zip") {
		err := UnzipZip("/storage/ita_uploads/"+fileName, dir)
		if err != nil {
			return errors.New("failed @ UnzipZip: " + err.Error())
		}
	} else {
		return errors.New("unexpected file extension: " + fileName)
	}
	//deploy
	// ensure mounts
	err := k8s_mountRetNfs(app, "/"+app, "/www/"+app, false, corev1.MountPropagationNone)
	if err != nil {
		return errors.New("failed @ k8s_mountRetNfs: " + err.Error())
	}

	return nil
}
