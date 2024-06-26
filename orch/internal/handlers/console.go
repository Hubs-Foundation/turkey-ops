package handlers

import (
	"main/internal"
	"net/http"
	"text/template"
)

type consoleCfg struct {
	UserEmail   string `json:"key"`
	UserPicture string `json:"subdomain"`
}

var Console = http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
	if r.URL.Path != "/console" {
		http.Error(w, http.StatusText(http.StatusNotFound), http.StatusNotFound)
		return
	}

	internal.Logger.Sugar().Debugf("dump headers: %v", r.Header)

	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	t, err := template.ParseFiles("./_statics/console.html")
	if err != nil {
		panic("failed to parse console.html template -- " + err.Error())
	}

	cfg := consoleCfg{
		UserEmail:   r.Header.Get("X-Forwarded-UserEmail"),
		UserPicture: r.Header.Get("X-Forwarded-UserPicture"),
	}

	c, err := r.Cookie(internal.CONST_SESSION_TOKEN_NAME)
	if err != nil {
		newCookie := internal.CreateNewSession()
		http.SetCookie(w, newCookie)
		t.Execute(w, cfg)
		return
	}

	sess := internal.CACHE.Load(c.Value)
	if sess == nil {
		http.SetCookie(w, internal.CreateNewSession())
		t.Execute(w, cfg)
		return
	}

	t.Execute(w, cfg)

})

// var RootHF = http.HandlerFunc(
// 	func(w http.ResponseWriter, r *http.Request) {
// 		if r.URL.Path != "/" {
// 			http.Error(w, http.StatusText(http.StatusNotFound), http.StatusNotFound)
// 			return
// 		}
// 		w.Header().Set("Content-Type", "text/html; charset=utf-8")
// 		t, err := template.ParseFiles("./root.html")
// 		if err != nil {
// 			internal.logger.Panic("failed to parse root.html template -- " + err.Error())
// 		}
// 		t.Execute(w, nil)
// 	})
