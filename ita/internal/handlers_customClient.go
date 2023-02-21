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
	if r.URL.Path != "/upload" && r.URL.Path != "/api/ita/upload" {
		http.Error(w, http.StatusText(http.StatusNotFound), http.StatusNotFound)
		return
	}
	if r.Method == "POST" {
		_, err := receiveFileFromReq(r, -1)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
		}
		reqId := w.Header().Get("X-Request-Id")
		fmt.Fprintf(w, "done, reqId: %v", reqId)
	}

})

//curl -X PATCH ita:6000/deploy/hubs?file=<name-of-the-file-under-/storage/ita-uploads>
var DeployHubs = http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
	if r.URL.Path != "/deploy/hubs" && r.URL.Path != "/api/ita/deploy/hubs" {
		http.Error(w, http.StatusText(http.StatusNotFound), http.StatusNotFound)
		return
	}

	if r.Method == "PATCH" {
		if len(r.URL.Query()["file"]) != 1 || r.URL.Query()["file"][0] == "" {
			http.Error(w, "missing: file", http.StatusBadRequest)
			return
		}
		fileName := r.URL.Query()["file"][0]

		err := unzipNdeployCustomHubs(fileName)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		w.WriteHeader(http.StatusOK)
		fmt.Fprint(w, "deployed: "+fileName)
		return
	}

	if r.Method == "POST" {
		files, err := receiveFileFromReq(r, 1)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
		}
		reqId := w.Header().Get("X-Request-Id")

		err = unzipNdeployCustomHubs(files[0])
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

var UndeployHubs = http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
	if r.URL.Path != "/undeploy/hubs" && r.URL.Path != "/api/ita/undeploy/hubs" {
		http.Error(w, http.StatusText(http.StatusNotFound), http.StatusNotFound)
		return
	}

	err := k8s_removeNfsMount("hubs")
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusOK)
	fmt.Fprintf(w, "done")
})

////////////////////////////////////////////////////////////////////////////////////////////////////

func unzipNdeployCustomHubs(fileName string) error {

	// if appName != "hubs" && appName != "spoke" {
	// 	return errors.New("bad appName: " + appName)
	// }

	os.RemoveAll("/storage/hubs/")

	//unzip
	if strings.HasSuffix(fileName, ".tar.gz") {
		err := UnzipTar("/storage/ita_uploads/"+fileName, "/storage/hubs/")
		if err != nil {
			return errors.New("failed @ UnzipTar: " + err.Error())
		}
	} else if strings.HasSuffix(fileName, ".zip") {
		err := UnzipZip("/storage/ita_uploads/"+fileName, "/storage/hubs/")
		if err != nil {
			return errors.New("failed @ UnzipZip: " + err.Error())
		}
	} else {
		return errors.New("unexpected file extension: " + fileName)
	}
	//deploy
	// ensure mounts
	err := k8s_mountRetNfs("hubs", "/hubs", "/www/hubs", false, corev1.MountPropagationNone)
	if err != nil {
		return errors.New("failed @ k8s_mountRetNfs: " + err.Error())
	}

	return nil
}
