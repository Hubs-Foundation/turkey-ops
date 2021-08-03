package handlers

import (
	"net/http"
	"text/template"

	"main/utils"
)

var Root = http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
	if r.URL.Path != "/" {
		http.Error(w, http.StatusText(http.StatusNotFound), http.StatusNotFound)
		return
	}

	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	t, err := template.ParseFiles("./_templates/root.html")
	if err != nil {
		utils.Logger.Panic("failed to parse root.html template -- " + err.Error())
	}

	c, err := r.Cookie(utils.SessionTokenName)
	if err != nil {
		newCookie := utils.CreateNewSession()
		http.SetCookie(w, newCookie)
		t.Execute(w, nil)
		return
	}

	sess := utils.CACHE.Load(c.Value)
	if sess == nil {
		http.SetCookie(w, utils.CreateNewSession())
		t.Execute(w, nil)
		return
	}

	t.Execute(w, nil)

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
