package handlers

import (
	"encoding/json"
	"fmt"
	"main/internal"
	"net/http"
)

var Dockerhub = http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
	decoder := json.NewDecoder(r.Body)
	var v map[string]interface{}
	err := decoder.Decode(&v)
	if err != nil {
		internal.GetLogger().Warn("bad r.Body")
		http.Error(w, http.StatusText(http.StatusNotFound), http.StatusNotFound)
		return
	}
	internal.GetLogger().Info("new dockerhub deployment")

	fmt.Println(dumpHeader(r))
	b, _ := json.MarshalIndent(v, "", "  ")
	fmt.Println(string(b))

	return
})
