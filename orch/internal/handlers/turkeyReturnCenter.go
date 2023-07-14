package handlers

import (
	"context"
	"fmt"
	"io/ioutil"
	"main/internal"
	"net/http"
	"strconv"
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

	for {
		mt, message, err := conn.ReadMessage()
		if err != nil {
			internal.Logger.Debug("err @ conn.ReadMessage:" + err.Error())
			break
		}
		strMessage := string(message)
		internal.Logger.Sugar().Debugf("recv: type=<%v>, msg=<%v>", mt, strMessage)

		if strMessage == "hi" {
			ctl_t0 := time.Now().UnixNano()
			tokenStr := fmt.Sprintf("token:%v", ctl_t0)
			err := internal.Cfg.Redis.Client().Set(context.Background(), "trc_"+subdomain, tokenStr, 1*time.Minute).Err()
			if err != nil {
				internal.Logger.Sugar().Errorf("failed to cache tokenStr: %v", err)
			}
			conn.WriteMessage(websocket.TextMessage, []byte(tokenStr))
			continue
		}

		sendMsg := "..."

		if strings.HasPrefix(strMessage, "token:") {
			// internal.Logger.Debug("strMessage: " + strMessage)
			tokenStr, err := internal.Cfg.Redis.Client().Get(context.Background(), "trc_"+subdomain).Result()
			if err != nil {
				internal.Logger.Sugar().Errorf("failed to retrieve tokenStr: %v", err)
				continue
			}
			// internal.Logger.Debug("tokenStr: " + tokenStr)

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
