package handlers

import (
	"context"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"main/internal"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/gorilla/websocket"
	"github.com/tanfarming/goutils/pkg/filelocker"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

var TurkeyReturnCenter = http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {

	subdomain := strings.Split(r.Host, ".")[0]
	internal.Logger.Sugar().Debugf("subdomain: %v", subdomain)

	//check if subdomain's collected
	hubId := internal.TrcCmBook.GetHubId(subdomain)
	if hubId == "" {
		http.Error(w, "", 404)
		return
	}

	if r.URL.Path == "/api-internal/v1/presence" {
		fmt.Fprint(w, `{"count":"-1"}`)
		return
	}
	if r.URL.Path == "/api-internal/v1/storage" {
		fmt.Fprint(w, `{"storage_mb":-1}`)
		return
	}

	if strings.HasSuffix(r.URL.Path, "/websocket") {
		trc_ws(w, r, subdomain, hubId)
		return
	}

	switch r.Method {
	case "GET":
		bytes, err := ioutil.ReadFile("./_statics/turkeyreturncenter.html")
		if err != nil {
			internal.Logger.Sugar().Errorf("%v", err)
		}
		fmt.Fprint(w, string(bytes))
	default:
		http.Error(w, "", 404)
	}
})

func trc_ws(w http.ResponseWriter, r *http.Request, subdomain, hubId string) {

	upgrader := websocket.Upgrader{
		ReadBufferSize:  1024,
		WriteBufferSize: 1024,
	}
	conn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		internal.Logger.Error("failed to upgrade: " + err.Error())
		http.Error(w, "", http.StatusInternalServerError)
		return
	}
	defer conn.Close()

	ctl_t0 := time.Now().UnixNano()
	tokenStr := fmt.Sprintf("token:%v", ctl_t0)
	for {
		mt, message, err := conn.ReadMessage()
		if err != nil {
			internal.Logger.Debug("err @ conn.ReadMessage:" + err.Error())
			break
		}
		strMessage := string(message)
		internal.Logger.Sugar().Debugf("recv: type=<%v>, msg=<%v>", mt, strMessage)

		if strMessage == "hi" {
			// err := internal.Cfg.Redis.Client().Set(context.Background(), "trc_"+subdomain, tokenStr, 1*time.Minute).Err()
			// if err != nil {
			// 	internal.Logger.Sugar().Errorf("failed to cache tokenStr: %v", err)
			// 	return
			// }
			conn.WriteMessage(websocket.TextMessage, []byte(tokenStr))
			continue
		}

		sendMsg := "..."

		if strings.HasPrefix(strMessage, "token:") {
			// // internal.Logger.Debug("strMessage: " + strMessage)
			// tokenStr, err := internal.Cfg.Redis.Client().Get(context.Background(), "trc_"+subdomain).Result()
			// if err != nil {
			// 	internal.Logger.Sugar().Errorf("failed to retrieve tokenStr: %v", err)
			// 	continue
			// }
			// // internal.Logger.Debug("tokenStr: " + tokenStr)

			if tokenStr != strMessage {
				internal.Logger.Sugar().Debugf("bad token, want <%v>, get <%v>", tokenStr, strMessage)
				continue
			}

			ctl_t0_str := strings.TrimPrefix(tokenStr, "token:")
			ctl_t0, _ := strconv.ParseInt(ctl_t0_str, 10, 64)
			dt := time.Since(time.Unix(ctl_t0/int64(time.Second), 0))

			internal.Logger.Sugar().Debugf("dt:%v, dt > 9*time.Second %v", dt, (dt > 9*time.Second))
			if dt > 9*time.Second {
				err := hc_restore(hubId)
				if err == nil {
					sendMsg = "_ok_"
				} else {
					internal.Logger.Sugar().Warn("failed @hc_restore: %v", err)
					if strings.HasPrefix(err.Error(), "***") {
						sendMsg = err.Error()[3:]
					}
				}
			} else {
				internal.Logger.Sugar().Debugf("bad dt: %v", dt)
				sendMsg = "-_-"
				internal.Logger.Sugar().Debugf("strMessage=<%v>, (need) tokenStr=<%v>", strMessage, tokenStr)
			}
		}
		err = conn.WriteMessage(mt, []byte(sendMsg))
		if err != nil {
			internal.Logger.Debug("err @ conn.WriteMessage:" + err.Error())
			break
		}

	}
}

func Cronjob_trcCacheBookSurveyor(interval time.Duration) {
	t0 := time.Now()
	rootDir := "/turkeyfs"

	// lock with a file, only 1 instance needed
	surveyorlockfile := rootDir + "/surveyorlock"
	f_surveyorlock, err := os.OpenFile(surveyorlockfile, os.O_WRONLY|os.O_CREATE, 0600)
	if filelocker.Lock(f_surveyorlock); err != nil {
		internal.Logger.Debug("failed to lock on the surveyorlock file: bail")
		return
	}
	defer f_surveyorlock.Close()
	surveyorlockfileBytes, err := os.ReadFile(surveyorlockfile)
	if err != nil {
		internal.Logger.Error("failed to read the surveyorlock file: bail")
		return
	}
	lastSurvey, err := time.Parse(time.RFC1123Z, string(surveyorlockfileBytes))
	if err != nil {
		internal.Logger.Warn("failed to parse timestamp in surveyorlockfile " + err.Error())
		lastSurvey = time.Now().Add(-100 * time.Hour)
	}
	if time.Since(lastSurvey) < 10*time.Minute {
		internal.Logger.Sugar().Debugf("skipping -- last surveyed (%v) is within (%v)", lastSurvey, time.Since(lastSurvey))
		return
	}

	hcNsList, err := internal.Cfg.K8ss_local.ClientSet.CoreV1().Namespaces().List(context.Background(), metav1.ListOptions{
		LabelSelector: "hub_id",
	})
	if err != nil {
		internal.Logger.Error("failed to get hcNsList")
	}
	nsMap := map[string]map[string]string{}
	for _, ns := range hcNsList.Items {
		nsMap[ns.Labels["hub_id"]] = ns.Labels
	}

	cutoffTime := internal.TrcCache.Updated_at.Add(-12 * time.Hour)
	_book := map[string]internal.TrcCacheData{}
	walkDirCnt := 0
	err = filepath.Walk(rootDir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		// internal.Logger.Sugar().Debugf("path: %v, info.IsDir(): %v, filepath.Dir(path): %v", path, info.IsDir(), filepath.Dir(path))
		// internal.Logger.Sugar().Debugf("strings.Count(filepath.Dir(path), string(os.PathSeparator)): %v", strings.Count(filepath.Dir(path), string(os.PathSeparator)))

		if strings.Count(filepath.Dir(path), string(os.PathSeparator)) > 1 {
			return filepath.SkipDir
		}

		// if info.IsDir() && filepath.Dir(path) != path {
		// 	return filepath.SkipDir
		// }

		internal.Logger.Sugar().Debugf("walking dir: %v", path)
		walkDirCnt++
		pathArr := strings.Split(path, "/")
		if !info.IsDir() {
			internal.Logger.Sugar().Debugf("skip -- not Dir, path : %v", path)
			return nil
		}
		if len(pathArr) < 3 || !strings.HasPrefix(pathArr[2], "hc-") {
			internal.Logger.Sugar().Debugf("skip -- not hc- folder, path: %v", path)
			return nil
		}
		internal.Logger.Sugar().Debugf("processing dir: %v", path)
		if info.ModTime().After(cutoffTime) {
			// get cfg
			hubId, _ := strings.CutPrefix(pathArr[2], "hc-")
			trc_cfg, err := GetHCcfgFromHubDir(hubId)
			if err != nil {
				internal.Logger.Sugar().Errorf("faild to get trc_cfg for hubId: %v", hubId)

			}
			IsHubRunning := nsMap[hubId] == nil
			_book[trc_cfg.Subdomain] = internal.TrcCacheData{
				HubId:        trc_cfg.HubId,
				OwnerEmail:   trc_cfg.UserEmail,
				IsRunning:    IsHubRunning,
				Collected_at: info.ModTime(),
			}
		}
		return nil
	})
	if err != nil {
		internal.Logger.Error("unexpected err during filepath.Walk: " + err.Error())
	}

	_bookBytes, err := json.Marshal(_book)
	if err != nil {
		internal.Logger.Error("failed to marshal _book: " + err.Error())
		return
	}

	// update the trcCache file
	f, err := os.OpenFile(internal.TrcCache.File, os.O_WRONLY, 0600)
	if err != nil {
		internal.Logger.Error("failed to open trcCache file")
		return
	}
	if err := filelocker.Lock(f); err != nil {
		internal.Logger.Error("failed to lock the trcCache file: " + err.Error())
		return
	}
	defer f.Close()
	f.Truncate(0)
	err = os.WriteFile(internal.TrcCache.File, _bookBytes, 0600)

	if err != nil {
		internal.Logger.Error("failed to write trcCache file: %" + err.Error())
		return
	}

	internal.Logger.Sugar().Debugf("walkDirCnt %v, took: %v", walkDirCnt, time.Since(t0))

	f_surveyorlock.Truncate(0)
	err = os.WriteFile(surveyorlockfile, []byte(time.Now().Format(time.RFC1123Z)), 0600)
	if err != nil {
		internal.Logger.Error("failed to update surveyorlockfile")
	}

}
