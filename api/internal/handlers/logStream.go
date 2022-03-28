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

	// check if user got a good cookie
	cookie, err := r.Cookie(internal.SessionTokenName)
	if err != nil {
		fmt.Fprintf(w, "??? who are you ???")
		return
	}

	sess := internal.CACHE.Load(cookie.Value)
	if sess == nil {
		fmt.Fprintf(w, "??? where's your cacheData ???")
		sess = internal.AddCacheData(cookie)
	}

	sess.SseChan = make(chan string)
	if len(sess.DeadLetterQueue) > 0 {
		sess.Log("&#127383; (logStream) <b>new</b> connection for sess: " + cookie.Value + " &#9193;" + r.RemoteAddr)
	} else {
		sess.Log("&#127383; (logStream) <b>re</b>connected for sess: " + cookie.Value + " &#9193;" + r.RemoteAddr)
		if len(sess.DeadLetterQueue) > 0 {
			sess.Log(fmt.Sprintf(" ###### poping %v messages in DeadLetterQueue ######", len(sess.DeadLetterQueue)))
			for i, m := range sess.DeadLetterQueue {
				sess.Log(fmt.Sprintf("msg #%v: %v", len(sess.DeadLetterQueue)-i, m))
			}
		}
	}

	notify := w.(http.CloseNotifier).CloseNotify()
	go func() {
		<-notify
		// internal.CACHE.Get(cookie.Value).SseChan = nil
		sess.SseChan = nil
		log.Println("HTTP connection just closed.")
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

	fmt.Println("connection established for:" + r.RemoteAddr + " @ " + cookie.Value)

	for {
		if sess.SseChan == nil {
			fmt.Println(" ??? LogStream: channel == nil ??? quit !!!")
			break
		}

		msg, has := <-sess.SseChan
		if has {

			fmt.Fprintf(w, "data: ["+time.Now().UTC().Format("2006.01.02-15:04:05")+"] -- "+msgBeautifier(msg)+"\n\n")
		}

		// fmt.Fprintf(w, "data: ["+time.Now().UTC().Format("2006.01.02-15:04:05")+"] -- %s\n\n", <-internal.CACHE.Get(cookie.Value).SseChan)

		f.Flush()
	}

	log.Println("Finished HTTP request at ", r.URL.Path)
})

func msgBeautifier(msg string) string {
	if strings.HasPrefix(msg, "ERROR") {
		msg = "&#128293;" + msg + "&#128293;"
	}
	if strings.HasPrefix(msg, "WARNING") {
		msg = "&#128576;" + msg + "&#128576;"
	}
	return msg
}
