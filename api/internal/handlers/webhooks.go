package handlers

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"main/internal"
	"net/http"
	"strconv"
	"strings"
)

var supportedChannels = map[string]bool{
	"dev":    true,
	"beta":   true,
	"stable": true,
}

// https://docs.docker.com/docker-hub/webhooks/#example-webhook-payload
type dockerhubWebhookJson struct {
	Callback_url string                          `json:"callback_url"`
	Push_data    dockerhubWebhookJson_push_data  `json:"push_data"`
	Repository   dockerhubWebhookJson_Repository `json:"repository"`
}
type dockerhubWebhookJson_push_data struct {
	Pusher string `json:"pusher"`
	Tag    string `json:"tag"`
}
type dockerhubWebhookJson_Repository struct {
	Repo_name string `json:"repo_name"`
}

var Dockerhub = http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
	if r.URL.Path != "/webhooks/dockerhub" || r.Method != "POST" {
		http.Error(w, http.StatusText(http.StatusNotFound), http.StatusNotFound)
		return
	}

	internal.Logger.Sugar().Debugf("dump headers: %v", r.Header)
	//++++++++++++++++++++++++
	//>>get bytes for debug print + decode
	// rBodyBytes, _ := ioutil.ReadAll(r.Body)
	// internal.Logger.Debug(prettyPrintJson(rBodyBytes))
	// decoder := json.NewDecoder(bytes.NewBuffer(rBodyBytes))
	//>>or if we don't need debug print:
	decoder := json.NewDecoder(r.Body)
	//-----------------------

	var dockerJson dockerhubWebhookJson
	err := decoder.Decode(&dockerJson)
	if err != nil || !strings.HasPrefix(dockerJson.Callback_url, "https://registry.hub.docker.com/u/mozillareality/") {
		internal.Logger.Debug(" bad r.Body, is it json? have they changed it? (https://docs.docker.com/docker-hub/webhooks/#example-webhook-payload)")
		http.Error(w, http.StatusText(http.StatusNotFound), http.StatusNotFound)
		return
	}

	//todo: verify ... docker's really lacking here, check with docker, maybe cross check with github action too?

	tagArr := strings.Split(dockerJson.Push_data.Tag, "-")

	//assume we can trust the payload at this point
	internal.Logger.Debug(fmt.Sprintf("parsed dockerJson: %+v", dockerJson))
	channel := tagArr[0]
	_, ok := supportedChannels[channel]
	if !ok {
		internal.Logger.Error("bad Channel: " + channel)
		http.Error(w, http.StatusText(http.StatusNotFound), http.StatusNotFound)
		return
	}
	//filter out utility tags
	if len(tagArr) < 2 {
		return
	}
	if _, err := strconv.Atoi(tagArr[1]); err != nil {
		return
	}

	err = updateGcsTurkeyBuildReportFile(channel, dockerJson.Repository.Repo_name, dockerJson.Push_data.Tag)
	if err != nil {
		internal.GetLogger().Error(err.Error())
	}

})

func updateGcsTurkeyBuildReportFile(channel, imgReponame, imgTag string) error {
	bucket := "turkeycfg"
	filename := "build-report-" + channel
	//read
	curr, err := internal.Cfg.Gcps.GCS_ReadFile(bucket, filename)
	if err != nil {
		return err
	}
	brMap := make(map[string]string)
	err = json.Unmarshal(curr, &brMap)
	if err != nil {
		return err
	}
	//update
	brMap[imgReponame] = imgTag
	//write
	brMapBytes, err := json.Marshal(brMap)
	if err != nil {
		return err
	}
	err = internal.Cfg.Gcps.GCS_WriteFile(bucket, filename, string(brMapBytes))
	if err != nil {
		return err
	}
	return nil
}

func prettyPrintJson(jsonBytes []byte) string {
	d := json.NewDecoder(bytes.NewBuffer(jsonBytes))
	var m map[string]interface{}
	_ = d.Decode(&m)
	b, _ := json.MarshalIndent(m, "", "  ")
	return string(b)
}

type ghaReport struct {
	Tag     string `json:"tag"`
	Channel string `json:"channel"`
}

// var GhaTurkey = http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
// 	if r.URL.Path != "/webhooks/ghaturkey" || r.Method != "POST" {
// 		http.Error(w, http.StatusText(http.StatusNotFound), http.StatusNotFound)
// 		return
// 	}
// 	//todo -- make an api key sort of thing for ddos protection?
// 	//todo -- doublecheck back against github and dockerhub?

// 	internal.Logger.Sugar().Debugf("dump headers: %v", r.Header)

// 	rBodyBytes, _ := ioutil.ReadAll(r.Body)
// 	internal.Logger.Debug(prettyPrintJson(rBodyBytes))
// 	decoder := json.NewDecoder(bytes.NewBuffer(rBodyBytes))
// 	//decoder := json.NewDecoder(r.Body)

// 	var ghaReport ghaReport
// 	err := decoder.Decode(&ghaReport)
// 	if err != nil {
// 		internal.Logger.Debug(" bad r.Body" + err.Error())
// 		http.Error(w, http.StatusText(http.StatusNotFound), http.StatusNotFound)
// 		return
// 	}

// 	_, ok := supportedChannels[ghaReport.Channel]
// 	if !ok {
// 		internal.Logger.Error("bad ghaReport.Channel: " + ghaReport.Channel)
// 		http.Error(w, http.StatusText(http.StatusNotFound), http.StatusNotFound)
// 		return
// 	}

// 	//publish
// 	TagArr := strings.Split(ghaReport.Tag, ":")
// 	if len(TagArr) != 2 {
// 		internal.Logger.Error("bad ghaReport.Tag: " + ghaReport.Tag)
// 		http.Error(w, http.StatusText(http.StatusNotFound), http.StatusNotFound)
// 		return
// 	}

// 	err = publishToConfigmap_label("hubsbuilds-"+ghaReport.Channel, TagArr[0], TagArr[1])
// 	if err != nil {
// 		internal.Logger.Error(err.Error())
// 	}

// 	http.Error(w, http.StatusText(http.StatusNotFound), http.StatusNotFound)
// })

var TurkeyGitops = http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
	if r.URL.Path != "/webhooks/turkeygitops" || r.Method != "POST" {
		http.Error(w, http.StatusText(http.StatusNotFound), http.StatusNotFound)
		return
	}

	internal.Logger.Sugar().Debugf("dump headers: %v", r.Header)
	rBodyBytes, _ := ioutil.ReadAll(r.Body)
	internal.Logger.Debug(prettyPrintJson(rBodyBytes))

})
