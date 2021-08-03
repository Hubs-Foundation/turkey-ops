package handlers

import (
	"fmt"
	"log"
	"main/utils"
	"net/http"
	"strings"
	"time"
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
	cookie, err := r.Cookie(utils.SessionTokenName)
	if err != nil {
		fmt.Fprintf(w, "??? who are you ???")
		return
	}

	sess := utils.CACHE.Load(cookie.Value)
	if sess == nil {
		fmt.Fprintf(w, "??? where's your cacheData ???")
		sess = utils.AddCacheData(cookie)
	}

	sess.SseChan = make(chan string)
	sess.PushMsg("&#127383; LogStream connected for sess: " + cookie.Value + " &#9193;" + r.RemoteAddr)

	notify := w.(http.CloseNotifier).CloseNotify()
	go func() {
		<-notify
		// utils.CACHE.Get(cookie.Value).SseChan = nil
		sess.SseChan = nil
		log.Println("HTTP connection just closed.")
	}()

	// //vvvvvvvvvvvvvvvvvvvvv junk log producer for debugging vvvvvvvvvvvvvvvvvvvvvvvvvvvv
	// go func() {
	// 	for i := 0; ; i++ {
	// 		// utils.CACHE.Get(cookie.Value).SseChan <- fmt.Sprintf("%d -- @ %v", i, "hello")
	// 		utils.CACHE.Get(cookie.Value).PushMsg(fmt.Sprintf("%d -- @ %v", i, "hello"))
	// 		log.Printf("junk msg #%d ", i)
	// 		time.Sleep(3e9)
	// 	}
	// }()
	// //^^^^^^^^^^^^^^^^^^^^^ junk log producer for debugging ^^^^^^^^^^^^^^^^^^^^^^^^^

	utils.Logger.Println("connection established for:" + r.RemoteAddr + " @ " + cookie.Value)

	for {
		if sess.SseChan == nil {
			utils.Logger.Println(" ??? LogStream: channel == nil ??? quit !!!")
			break
		}

		msg, has := <-sess.SseChan
		if has {

			fmt.Fprintf(w, "data: ["+time.Now().UTC().Format("2006.01.02-15:04:05")+"] -- "+msgBeautifier(msg)+"\n\n")
		}

		// fmt.Fprintf(w, "data: ["+time.Now().UTC().Format("2006.01.02-15:04:05")+"] -- %s\n\n", <-utils.CACHE.Get(cookie.Value).SseChan)

		f.Flush()
	}

	log.Println("Finished HTTP request at ", r.URL.Path)
})

func msgBeautifier(msg string) string {
	if strings.Contains(msg, "ERROR") {
		msg = "&#128293;" + msg + "&#128293;"
	}
	return msg
}
