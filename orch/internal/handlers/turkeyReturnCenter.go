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

var _resuming_status = int32(0)

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
		trc_ws(w, r)
	}

	switch r.Method {
	case "GET":
		// if _, ok := r.Header["Duck"]; ok {
		// 	duckpng, _ := os.Open("./_statics/duck.png")
		// 	defer duckpng.Close()
		// 	img, _, _ := image.Decode(duckpng)
		// 	rand.Seed(time.Now().UnixNano())
		// 	rotatedImg := rotateImg(img, 25+rand.Float64()*250)
		// 	var buffer bytes.Buffer
		// 	_ = png.Encode(&buffer, rotatedImg)
		// 	encoded := base64.StdEncoding.EncodeToString(buffer.Bytes())
		// 	fmt.Fprint(w, encoded)
		// 	return
		// }
		bytes, err := ioutil.ReadFile("./_statics/turkeyreturncenter.html")
		if err != nil {
			internal.Logger.Sugar().Errorf("%v", err)
		}
		fmt.Fprint(w, string(bytes))
	case "PUT":
		if _resuming_status != 0 {
			http.Error(w, "", http.StatusBadRequest)
			return
		}
		// err := HC_Resume()	<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<
		// if err != nil {
		// 	internal.Logger.Sugar().Errorf("err (reqId: %v): %v", w.Header().Get("X-Request-Id"), err)
		// 	fmt.Fprintf(w, "something went wrong -- reqId=%v", w.Header().Get("X-Request-Id"))
		// 	return
		// }
		fmt.Fprint(w, "resuming")
	case "UPDATE":
		internal.Logger.Sugar().Debugf("_resuming_status: %v", _resuming_status)
		if _resuming_status == 0 {
			fmt.Fprint(w, "slide to fix the ducks orintation")
			return
		}
		if _resuming_status < 0 {
			fmt.Fprintf(w, "resuming, this can take a few minutes")
			return
		}
		fmt.Fprintf(w, "not ready yet, try again in %v min", (_resuming_status / 60))
	default:
		http.Error(w, "", 404)
	}
})

func trc_ws(w http.ResponseWriter, r *http.Request) {

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
			internal.Logger.Sugar().Debugf("_resuming_status: %v", _resuming_status)
			time.Sleep(1 * time.Second)
			if _resuming_status == 0 {
				continue
			}
			sendMsg := fmt.Sprintf("cooldown in progress -- try again in %v min", (_resuming_status / 60))
			if _resuming_status < 0 {
				watingMsg += "."
				sendMsg = watingMsg
			}

			resume_cooldown_sec := 86400 // 1 day
			if (_resuming_status) > int32(resume_cooldown_sec-900) {
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
		if strMessage == "hi" && _resuming_status == 0 {
			conn.WriteMessage(websocket.TextMessage, []byte("hi"))
		}
		if strings.HasPrefix(strMessage, "_r_:") && _resuming_status == 0 {
			// HC_Resume()	<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<
			err = conn.WriteMessage(mt, []byte("respawning hubs pods"))
			if err != nil {
				internal.Logger.Debug("err @ conn.WriteMessage:" + err.Error())
				break
			}
		}
	}
}
