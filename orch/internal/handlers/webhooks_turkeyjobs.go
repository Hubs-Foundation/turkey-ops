package handlers

import (
	"context"
	"encoding/json"
	"io/ioutil"
	"main/internal"
	"net/http"
)

var Webhook_turkeyJobs = http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
	if r.Method == "POST" {
		handleTurkeyJobCallback(r)
	}
	http.Error(w, http.StatusText(http.StatusNotFound), http.StatusNotFound)
})

func handleTurkeyJobCallback(r *http.Request) {
	rBodyBytes, err := ioutil.ReadAll(r.Body)
	if err != nil {
		internal.Logger.Error("ERROR @ reading r.body, error = " + err.Error())
		return
	}
	payload := map[string]string{}
	err = json.Unmarshal(rBodyBytes, &payload)
	if err != nil {
		internal.Logger.Sugar().Errorf("failed to unmarshalbad (%v), err: %v", string(rBodyBytes), err)
		return
	}

	if payload["id"] != "" {
		internal.Cfg.Redis.RPush(
			context.Background(),
			payload["id"], payload)
	}

}