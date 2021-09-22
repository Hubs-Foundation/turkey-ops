package handlers

import (
	"fmt"
	"net/http"
	"text/template"

	"main/utils"
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
	fmt.Println("dumpHeader: " + dumpHeader(r))

	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	t, err := template.ParseFiles("./_statics/console.html")
	if err != nil {
		utils.Logger.Panic("failed to parse console.html template -- " + err.Error())
		return
	}

	cfg := consoleCfg{
		UserEmail:   r.Header.Get("X-Forwarded-UserEmail"),
		UserPicture: r.Header.Get("X-Forwarded-UserPicture"),
	}

	c, err := r.Cookie(utils.SessionTokenName)
	if err != nil {
		newCookie := utils.CreateNewSession()
		http.SetCookie(w, newCookie)
		t.Execute(w, cfg)
		return
	}

	sess := utils.CACHE.Load(c.Value)
	if sess == nil {
		http.SetCookie(w, utils.CreateNewSession())
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
// 			utils.Logger.Panic("failed to parse root.html template -- " + err.Error())
// 		}
// 		t.Execute(w, nil)
// 	})
