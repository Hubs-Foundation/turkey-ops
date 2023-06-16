package handlers

import (
	"fmt"
	"log"
	"net/http"
	"strings"
	"time"

	"main/internal"
)

var LogStream = http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {

	if r.URL.Path != "/LogStream" {
		http.Error(w, http.StatusText(http.StatusNotFound), http.StatusNotFound)
		return
	}
	// Make sure that the writer supports flushing.
	f, ok := w.(http.Flusher)
	if !ok {
		http.Error(w, "Streaming unsupported!", http.StatusInternalServerError)
		return
	}

	// Set the headers related to event streaming.
	// w.Header().Set("Content-Type", "text/html")
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
	// w.Header().Set("Transfer-Encoding", "chunked")
	w.Header().Set("Access-Control-Allow-Origin", "*")

	// check if this is an ui session
	cookie, err := r.Cookie(internal.CONST_SESSION_TOKEN_NAME)
	if err != nil {
		fmt.Fprintf(w, "no session cookie")
		return
	}
	sess := internal.CACHE.Load(cookie.Value)
	if sess == nil {
		fmt.Fprintf(w, "no session")
		return
	}

	sess.SseChan = make(chan string)
	sess.Log("[DEBUG] (/logStream) connected for sess: " + cookie.Value + " &#9193;" + r.RemoteAddr)
	if len(sess.DeadLetters) > 0 {
		sess.Log(fmt.Sprintf(" ###### poping %v messages in DeadLetterQueue ######", len(sess.DeadLetters)))
		for i, m := range sess.DeadLetters {
			sess.Log(fmt.Sprintf("msg #%v: %v", len(sess.DeadLetters)-i, m))
		}
	}

	// notify := w.(http.CloseNotifier).CloseNotify()
	// go func() {
	// 	<-notify
	// 	sess.SseChan = nil
	// 	internal.Logger.Debug("connection closed.")
	// }()
	go func() {
		select {
		case <-r.Context().Done():
			sess.SseChan = nil
			internal.Logger.Debug("session dropped because connection was closed.")
			return
		default:
		}
	}()

	// //vvvvvvvvvvvvvvvvvvvvv junk log producer for debugging vvvvvvvvvvvvvvvvvvvvvvvvvvvv
	// go func() {
	// 	for i := 0; ; i++ {
	// 		// internal.CACHE.Get(cookie.Value).SseChan <- fmt.Sprintf("%d -- @ %v", i, "hello")
	// 		internal.CACHE.Get(cookie.Value).PushMsg(fmt.Sprintf("%d -- @ %v", i, "hello"))
	// 		log.Printf("junk msg #%d ", i)
	// 		time.Sleep(3e9)
	// 	}
	// }()
	// //^^^^^^^^^^^^^^^^^^^^^ junk log producer for debugging ^^^^^^^^^^^^^^^^^^^^^^^^^
	internal.Logger.Debug("connection established for:" + r.RemoteAddr + " @ " + cookie.Value)

	for {
		if sess.SseChan == nil {
			fmt.Println(" ??? LogStream: channel == nil ??? quit !!!")
			break
		}
		msg, has := <-sess.SseChan
		if has {
			fmt.Fprintf(w, "data: "+msgBeautifier(msg)+"\n\n")
		}
		f.Flush()
	}
	log.Println("Finished HTTP request at ", r.URL.Path)
})

func msgBeautifier(msg string) string {
	msg = "[sse @ " + time.Now().UTC().Format("2006.01.02-15:04:05") + "] " + msg

	if strings.Contains(msg, "[ERROR]") {
		msg = "&#128293;" + msg + "&#128293;"
	}
	if strings.Contains(msg, "[WARN]") {
		msg = "&#128576;" + msg + "&#128576;"
	}
	if strings.Contains(msg, "[DEBUG]") {
		msg = "<span style=\"color:Gray;\">" + msg + "</span>"
	}
	return msg
}
