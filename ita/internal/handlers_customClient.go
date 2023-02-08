package internal

import (
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
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

//curl -X POST -F file='@<path-to-file-ie-/tmp/file1>' ita:6000/upload
func receiveFileFromReq(r *http.Request, expectedFileCount int) ([]string, error) {

	// 32 MB
	if err := r.ParseMultipartForm(32 << 20); err != nil {
		// http.Error(w, err.Error(), http.StatusBadRequest)
		return nil, err
	}

	Logger.Sugar().Debugf("r.MultipartForm.File: %v", r.MultipartForm.File)
	// get a reference to the fileHeaders
	files := r.MultipartForm.File["file"]

	if expectedFileCount != -1 && len(files) != expectedFileCount {
		return nil, errors.New("unexpected file count")
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
		result = append(result, fileHeader.Filename)
	}

	Logger.Sugar().Debugf("report: %v", report)
	return result, nil
}

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
	err := k8s_mountRetNfs("hubs", "/hubs", "/www/hubs")
	if err != nil {
		return errors.New("failed @ k8s_mountRetNfs: " + err.Error())
	}

	return nil
}
