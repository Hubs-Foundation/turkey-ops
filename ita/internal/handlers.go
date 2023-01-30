package internal

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
	"sync/atomic"
	"time"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func Healthz() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if atomic.LoadInt32(&Healthy) == 1 {
			w.WriteHeader(http.StatusNoContent)
			return
		}
		w.WriteHeader(http.StatusServiceUnavailable)
	})
}

var Ita_admin_info = http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
	resp := map[string]interface{}{
		"ssh_totp_qr_data":     "N/A",
		"ses_max_24_hour_send": 99999,
		"using_ses":            true,
		"worker_domain":        "N/A",
		"assets_domain":        "N/A",
		"server_domain":        cfg.Domain,
		"provider":             "N/A",
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(resp)
	return
})

var Ita_cfg_ret_ps = http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
	resp := map[string]interface{}{}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(resp)
	return
})

// var HC_launch_fallback = http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
// 	if r.URL.Path != "/hc_launch_fallback" || r.Method != "GET" {
// 		http.Error(w, http.StatusText(http.StatusNotFound), http.StatusNotFound)
// 		return
// 	}
// 	Logger.Debug(dumpHeader(r))

// 	fmt.Fprintf(w, "wip")
// 	return
// })

// var supportedChannels = map[string]bool{
// 	"dev":    true,
// 	"beta":   true,
// 	"stable": true,
// }
var Updater = http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
	if r.URL.Path != "/updater" {
		http.Error(w, http.StatusText(http.StatusNotFound), http.StatusNotFound)
		return
	}
	if r.Method == "POST" {
		if len(r.URL.Query()["channel"]) != 1 || r.URL.Query()["channel"][0] == "" {
			http.Error(w, "missing: channel", http.StatusBadRequest)
			return
		}
		channel := r.URL.Query()["channel"][0]
		// _, ok := cfg.SupportedChannels[channel]
		// if !ok {
		// 	Logger.Error("bad channel: " + channel)
		// 	http.Error(w, "bad channel: "+channel, http.StatusBadRequest)
		// 	return
		// }

		cfg.TurkeyUpdater.Start(channel)
		Set_listeningChannelLabel(channel) //persist to k8s-deployment-label to recover across pod reboot

		w.WriteHeader(200)
		return
	}
	if r.Method == "GET" {
		fmt.Fprint(w, cfg.TurkeyUpdater.Channel(), " --> ", cfg.TurkeyUpdater.Containers())
		return
	}
	http.Error(w, http.StatusText(http.StatusMethodNotAllowed), http.StatusMethodNotAllowed)

})

func HubInfraStatus() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {

		deployments, err := cfg.K8sClientSet.AppsV1().Deployments(cfg.PodNS).List(context.Background(), metav1.ListOptions{})
		if err != nil {
			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(map[string]interface{}{"status": "error: " + err.Error()})
			return
		}
		for _, d := range deployments.Items {
			if k8s_isDeploymentRunning(&d) {
				w.Header().Set("Content-Type", "application/json")
				json.NewEncoder(w).Encode(map[string]interface{}{"status": "deploying"})
				return
			}
		}

		pods, err := cfg.K8sClientSet.CoreV1().Pods(cfg.PodNS).List(context.Background(), metav1.ListOptions{})
		if err != nil {
			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(map[string]interface{}{"status": "error: " + err.Error()})
			return
		}
		if err := k8s_waitForPods(pods, 1*time.Second); err != nil {
			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(map[string]interface{}{"status": "podsPending"})
			return
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]interface{}{"status": "ready"})
		return
	})
}

var ClusterIps = http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
	if r.URL.Path != "/z/meta/cluster-ips" {
		http.Error(w, http.StatusText(http.StatusNotFound), http.StatusNotFound)
		return
	}
	if r.Method == "GET" {
		w.Header().Set("Content-Type", "application/json")
		res := StreamNodes
		json.NewEncoder(w).Encode(res)
		return
	}
	http.Error(w, http.StatusText(http.StatusNotFound), http.StatusNotFound)

})
var ClusterIpsList = http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
	if r.URL.Path != "/z/meta/cluster-ips/list" {
		http.Error(w, http.StatusText(http.StatusNotFound), http.StatusNotFound)
		return
	}
	if r.Method == "GET" {
		w.Header().Set("Cache-Control", "no-store, must-revalidate")
		w.Header().Set("Pragma", "no-cache")
		w.Header().Set("Expires", "0")
		res := StreamNodeIpList
		fmt.Fprint(w, res)
		return
	}
	http.Error(w, http.StatusText(http.StatusNotFound), http.StatusNotFound)

})

const MAX_UPLOAD_SIZE = 150 * 1024 * 1024 // 150mb
// Progress is used to track the progress of a file upload.It implements the io.Writer interface so it can be passed to an io.TeeReader()
type Progress struct {
	TotalSize int64
	BytesRead int64
}

// Write is used to satisfy the io.Writer interface. Instead of writing somewhere, it simply aggregates the total bytes on each read
func (pg *Progress) Write(p []byte) (n int, err error) {
	n, err = len(p), nil
	pg.BytesRead += int64(n)
	pg.Print()
	return
}

// Print displays the current progress of the file upload
func (pg *Progress) Print() {
	if pg.BytesRead == pg.TotalSize {
		Logger.Sugar().Debugf("DONE! %v", pg)
		return
	}

	fmt.Printf("File upload in progress: %d\n", pg.BytesRead)
}

var Upload = http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
	if r.URL.Path != "/upload" && r.URL.Path != "/ita/upload" {
		http.Error(w, http.StatusText(http.StatusNotFound), http.StatusNotFound)
		return
	}
	if r.Method == "POST" {
		_, report, err := receiveFileFromReq(r, -1)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
		}
		reqId := w.Header().Get("X-Request-Id")
		fmt.Fprintf(w, "received [reqId: %v]:\n %v", reqId, report)
	}

})

//curl -X POST -F file='@<path-to-file-ie-/tmp/file1>' ita:6000/upload
func receiveFileFromReq(r *http.Request, expectedFileCount int) ([]string, string, error) {

	// 32 MB
	if err := r.ParseMultipartForm(32 << 20); err != nil {
		// http.Error(w, err.Error(), http.StatusBadRequest)
		return nil, "", err
	}

	Logger.Sugar().Debugf("r.MultipartForm.File: %v", r.MultipartForm.File)
	// get a reference to the fileHeaders
	files := r.MultipartForm.File["file"]

	if expectedFileCount != -1 && len(files) != expectedFileCount {
		return nil, "", errors.New("unexpected file count")
	}

	result := []string{}
	report := ""
	for _, fileHeader := range files {
		fileHeader = files[0]
		if fileHeader.Size > MAX_UPLOAD_SIZE {
			report += fmt.Sprintf("skipped(too big): %v(%v/%vMB)\n", fileHeader.Filename, fileHeader.Size, MAX_UPLOAD_SIZE/(1048576))
			result = append(result, "(skipped)"+fileHeader.Filename)
			continue
		}
		Logger.Sugar().Debugf("working on file: %v (%v)", fileHeader.Filename, fileHeader.Size)
		file, err := fileHeader.Open()
		if err != nil {
			report += fmt.Sprintf("failed to open %v, err: %v \n", fileHeader.Filename, err)
			result = append(result, "(failed to open)"+fileHeader.Filename)
			continue
		}
		defer file.Close()
		buff := make([]byte, 512)
		_, err = file.Read(buff)
		if err != nil {
			report += fmt.Sprintf("failed to read %v, err: %v \n", fileHeader.Filename, err)
			result = append(result, "(failed to read)"+fileHeader.Filename)
			continue
		}
		filetype := http.DetectContentType(buff)

		Logger.Debug("filetype: " + filetype)

		_, err = file.Seek(0, io.SeekStart)
		if err != nil {
			report += fmt.Sprintf("failed to seek %v, err: %v \n", fileHeader.Filename, err)
			result = append(result, "(failed to seek)"+fileHeader.Filename)
			continue
		}
		err = os.MkdirAll("/storage/ita_uploads", os.ModePerm)
		if err != nil {
			report += fmt.Sprintf("failed to makeDir %v, err: %v \n", fileHeader.Filename, err)
			result = append(result, "(failed to makeDir)"+fileHeader.Filename)
			continue
		}
		f, err := os.Create(fmt.Sprintf("/storage/ita_uploads/%s", fileHeader.Filename))
		if err != nil {
			report += fmt.Sprintf("failed to create %v, err: %v \n", fileHeader.Filename, err)
			result = append(result, "(failed to create)"+fileHeader.Filename)
			continue
		}
		defer f.Close()

		pg := &Progress{
			TotalSize: fileHeader.Size,
		}
		_, err = io.Copy(f, io.TeeReader(file, pg))
		if err != nil {
			report += fmt.Sprintf("failed to copy %v, err: %v \n", fileHeader.Filename, err)
			result = append(result, "(failed to copy)"+fileHeader.Filename)
			continue
		}
		report += fmt.Sprintf("saved: %v(%v, %vMB)\n", f.Name(), filetype, fileHeader.Size/(1024*1024))
		result = append(result, f.Name())
	}

	return result, report, nil
}

//curl -X PATCH ita:6000/deploy/hubs?file=<name-of-the-file-under-/storage/ita-uploads>
var DeployHubs = http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
	if r.URL.Path != "/deploy/hubs" && r.URL.Path != "/ita/deploy/hubs" {
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
		// if len(r.URL.Query()["file"]) != 1 || r.URL.Query()["file"][0] == "" {
		// 	http.Error(w, "missing: file", http.StatusBadRequest)
		// 	return
		// }
		// fileName := r.URL.Query()["file"][0]
		files, report, err := receiveFileFromReq(r, 1)
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
		report += "deployed: " + files[0]
		fmt.Fprintf(w, "done [reqId: %v]:\n %v", reqId, report)
		return
	}

	http.Error(w, http.StatusText(http.StatusMethodNotAllowed), http.StatusMethodNotAllowed)

})

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
	//deploy (mount)
	err := k8s_mountRetNfs("hubs", "/hubs", "/www/hubs")
	if err != nil {
		return errors.New("failed @ k8s_mountRetNfs: " + err.Error())
	}

	return nil
}
