package handlers

import (
	"fmt"
	"io/ioutil"
	"main/internal"
	"net/http"
	"strings"
	"time"

	"github.com/gorilla/websocket"
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

	if strings.HasSuffix(r.URL.Path, "/websocket") {
		trc_ws(w, r, subdomain, hubId)
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
		internal.Logger.Sugar().Debugf("recv: type=<%v>, msg=<%v>", mt, string(strMessage))
		if strMessage == "hi" {
			conn.WriteMessage(websocket.TextMessage, []byte(tokenStr))
			continue
		}

		sendMsg := "..."

		if strings.HasPrefix(strMessage, "token:") {
			dt := time.Since(time.Unix(ctl_t0/int64(time.Second), 0))
			internal.Logger.Sugar().Debugf("dt: %v", dt)
			if strMessage == tokenStr && dt > 8*time.Second {
				err := hc_restore(hubId)
				if err == nil {
					sendMsg = "restoring hub instance, this may take a few minutes"
				} else if strings.HasPrefix(err.Error(), "***") {
					sendMsg = err.Error()
				}
			} else {
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
