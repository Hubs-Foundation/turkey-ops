package handlers

import (
	"encoding/json"
	"fmt"
	"main/internal"
	"net/http"
	"strings"
)

type dockerhubWebhookJson struct {
	Callback_url string
	Push_data    dockerhubWebhookJson_push_data
}

type dockerhubWebhookJson_push_data struct {
	Pusher string
	Tag    string
}

var Dockerhub = http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
	if r.URL.Path != "/webhooks/dockerhub" || r.Method != "POST" {
		http.Error(w, http.StatusText(http.StatusNotFound), http.StatusNotFound)
		return
	}

	decoder := json.NewDecoder(r.Body)
	var v dockerhubWebhookJson
	err := decoder.Decode(&v)
	if err != nil || !strings.HasPrefix(v.Callback_url, "https://registry.hub.docker.com/u/mozillareality/") {
		internal.GetLogger().Warn("bad r.Body" + err.Error())
		http.Error(w, http.StatusText(http.StatusNotFound), http.StatusNotFound)
		return
	}

	internal.GetLogger().Debug(fmt.Sprintf("%+v", v))

	if strings.HasPrefix(v.Push_data.Tag, "dev-") ||
		strings.HasPrefix(v.Push_data.Tag, "staging-") ||
		strings.HasPrefix(v.Push_data.Tag, "prod-") {
		internal.GetLogger().Info("deploying: " + v.Push_data.Tag)
	}

	// fmt.Println(dumpHeader(r))
	// b, _ := json.MarshalIndent(v, "", "  ")
	// fmt.Println(string(b))

})
