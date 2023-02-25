package internal

import (
	"errors"
	"fmt"
	"net/http"
	"os"
	"strings"

	corev1 "k8s.io/api/core/v1"
)

var Upload = http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
	features := cfg.Features.Get()
	if !features.customClient || (r.URL.Path != "/upload" && r.URL.Path != "/api/ita/upload") {
		http.Error(w, http.StatusText(http.StatusNotFound), http.StatusNotFound)
		return
	}
	if r.Method == "POST" {
		_, err := receiveFileFromReqForm(r, 1)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
		}
		reqId := w.Header().Get("X-Request-Id")
		fmt.Fprintf(w, "done, reqId: %v", reqId)
	}

})

//curl -X PATCH ita:6000/deploy?app=hubs?file=<name-of-the-file-under-/storage/ita-uploads>
var Deploy = http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
	features := cfg.Features.Get()
	if !features.customClient || (r.URL.Path != "/deploy" && r.URL.Path != "/api/ita/deploy") {
		http.Error(w, http.StatusText(http.StatusNotFound), http.StatusNotFound)
		return
	}
	app := r.URL.Query().Get("app")
	if app != "hubs" && app != "spoke" {
		http.Error(w, http.StatusText(http.StatusBadRequest), http.StatusBadRequest)
		return
	}

	if r.Method == "PATCH" {
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
		return
	}

	if r.Method == "POST" {
		files, err := receiveFileFromReqForm(r, 1)
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
		return
	}
	http.Error(w, http.StatusText(http.StatusMethodNotAllowed), http.StatusMethodNotAllowed)
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
