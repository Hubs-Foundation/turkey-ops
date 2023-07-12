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

var cooldown = 12 * time.Hour

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
	// case "PUT":
	// 	fmt.Fprint(w, "PUT")
	// case "UPDATE":
	// 	fmt.Fprintf(w, "UPDATE")
	default:
		http.Error(w, "", 404)
	}
})

func trc_ws(w http.ResponseWriter, r *http.Request, subdomain, hubId string) {

	watingMsg := "waiting for backends"

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
	go func() {
		//status report during HC_Resume(), incl cooldown period
		for {
			lastUsed := internal.TrcCmBook.GetLastUsed(subdomain)
			timeSinceLastUsed := time.Since(lastUsed)
			internal.Logger.Sugar().Debugf("lastUsed: %v", lastUsed)
			time.Sleep(1 * time.Second)
			if timeSinceLastUsed < cooldown {
				continue
			}
			sendMsg := fmt.Sprintf("cooldown in progress -- try again in %v min", (cooldown - timeSinceLastUsed).Minutes())
			if timeSinceLastUsed < 1*time.Minute {
				watingMsg += "."
				sendMsg = watingMsg
			} else if timeSinceLastUsed < 5*time.Minute {
				sendMsg = "_refresh_"
			}
			internal.Logger.Debug("sendMsg: " + sendMsg)
			err := conn.WriteMessage(websocket.TextMessage, []byte(sendMsg))
			if err != nil {
				internal.Logger.Debug("err @ conn.WriteMessage:" + err.Error())
				break
			}
			time.Sleep(10 * time.Second)
		}
	}()

	for {
		mt, message, err := conn.ReadMessage()
		if err != nil {
			internal.Logger.Debug("err @ conn.ReadMessage:" + err.Error())
			break
		}
		strMessage := string(message)
		internal.Logger.Sugar().Debugf("recv: type=<%v>, msg=<%v>", mt, string(strMessage))
		if strMessage == "hi" {
			conn.WriteMessage(websocket.TextMessage, []byte("hi"))
		}

		sendMsg := "???"

		lastUsed := internal.TrcCmBook.GetLastUsed(subdomain)
		timeSinceLastUsed := time.Since(lastUsed)
		if strings.HasPrefix(strMessage, "_r_:") {
			if timeSinceLastUsed > cooldown {
				sendMsg = "respawning hubs pods"
				internal.TrcCmBook.RecUsage(subdomain)
				hc_restore(hubId)
			}
			err = conn.WriteMessage(mt, []byte(sendMsg))
			if err != nil {
				internal.Logger.Debug("err @ conn.WriteMessage:" + err.Error())
				break
			}
		}

	}
}
