package handlers

import (
	"encoding/json"
	"fmt"
	"main/internal"
	"net/http"
	"strings"
)

// https://docs.docker.com/docker-hub/webhooks/#example-webhook-payload
type dockerhubWebhookJson struct {
	Callback_url string
	Push_data    dockerhubWebhookJson_push_data
	Repository   dockerhubWebhookJson_Repository
}
type dockerhubWebhookJson_push_data struct {
	Pusher string
	Tag    string
}
type dockerhubWebhookJson_Repository struct {
	repo_name string
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
		internal.GetLogger().Warn(" bad r.Body, is it json? ")
		http.Error(w, http.StatusText(http.StatusNotFound), http.StatusNotFound)
		return
	}

	internal.GetLogger().Debug(fmt.Sprintf("%+v", v))

	if strings.HasPrefix(v.Push_data.Tag, "dev-") ||
		strings.HasPrefix(v.Push_data.Tag, "staging-") ||
		strings.HasPrefix(v.Push_data.Tag, "prod-") {
		tag := v.Repository.repo_name + ":" + v.Push_data.Tag
		internal.GetLogger().Info("deploying: " + tag)
	}

})
