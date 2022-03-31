package handlers

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"main/internal"
	"net/http"
	"strings"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

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

type ghaReport struct {
	Tag     string `json:"tag"`
	Channel string `json:"channel"`
}

var supportedChannels = map[string]bool{
	"dev":    true,
	"beta":   true,
	"stable": true,
}

var GhaTurkey = http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
	if r.URL.Path != "/webhooks/ghaturkey" || r.Method != "POST" {
		http.Error(w, http.StatusText(http.StatusNotFound), http.StatusNotFound)
		return
	}
	//check api key for ddos protection?

	internal.GetLogger().Sugar().Debugf("dump headers: %v", r.Header)

	//++++++++++++++++++++++++

	//get bytes for debug print + decode
	rBodyBytes, _ := ioutil.ReadAll(r.Body)
	internal.GetLogger().Debug(prettyPrintJson(rBodyBytes))
	decoder := json.NewDecoder(bytes.NewBuffer(rBodyBytes))

	//or if we don't need debug print:
	//decoder := json.NewDecoder(r.Body)

	//-----------------------
	var ghaReport ghaReport
	err := decoder.Decode(&ghaReport)
	if err != nil {
		internal.GetLogger().Debug(" bad r.Body" + err.Error())
		http.Error(w, http.StatusText(http.StatusNotFound), http.StatusNotFound)
		return
	}

	_, ok := supportedChannels[ghaReport.Channel]
	if !ok {
		internal.GetLogger().Error("bad ghaReport.Channel: " + ghaReport.Channel)
		http.Error(w, http.StatusText(http.StatusNotFound), http.StatusNotFound)
		return
	}

	//publish
	TagArr := strings.Split(ghaReport.Tag, ":")
	if len(TagArr) != 2 {
		internal.GetLogger().Error("bad ghaReport.Tag: " + ghaReport.Tag)
		http.Error(w, http.StatusText(http.StatusNotFound), http.StatusNotFound)
		return
	}

	err = publishToConfigmap_label("hubsbuilds-"+ghaReport.Channel, TagArr[0], TagArr[1])
	if err != nil {
		internal.GetLogger().Error(err.Error())
	}
	// err = publishToConfigmap_data("hubsbuilds", ghaReport.Channel, TagArr[0], TagArr[1])
	// if err != nil {
	// 	internal.GetLogger().Error(err.Error())
	// }
	// err = publishToNamespaceTag(ghaReport.Channel, TagArr[0], TagArr[1])
	// if err != nil {
	// 	internal.GetLogger().Error("publishToNamespaceTag failed: " + err.Error())
	// }
	http.Error(w, http.StatusText(http.StatusNotFound), http.StatusNotFound)
})

var Dockerhub = http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
	if r.URL.Path != "/webhooks/dockerhub" || r.Method != "POST" {
		http.Error(w, http.StatusText(http.StatusNotFound), http.StatusNotFound)
		return
	}

	internal.GetLogger().Sugar().Debugf("dump headers: %v", r.Header)
	//++++++++++++++++++++++++
	//get bytes for debug print + decode
	rBodyBytes, _ := ioutil.ReadAll(r.Body)
	internal.GetLogger().Debug(prettyPrintJson(rBodyBytes))
	decoder := json.NewDecoder(bytes.NewBuffer(rBodyBytes))
	//or if we don't need debug print:
	//decoder := json.NewDecoder(r.Body)
	//-----------------------

	var dockerJson dockerhubWebhookJson
	err := decoder.Decode(&dockerJson)
	if err != nil || !strings.HasPrefix(dockerJson.Callback_url, "https://registry.hub.docker.com/u/mozillareality/") {
		internal.GetLogger().Debug(" bad r.Body, is it json? have they changed it? (https://docs.docker.com/docker-hub/webhooks/#example-webhook-payload)")
		http.Error(w, http.StatusText(http.StatusNotFound), http.StatusNotFound)
		return
	}

	//todo: verify ... docker's really lacking here, maybe check with docker and then cross check with github action?

	//assume we can trust the payload at this point
	internal.GetLogger().Debug(fmt.Sprintf("parsed dockerJson: %+v", dockerJson))
	channel := ""
	if dockerJson.Push_data.Tag == "dev-" {
		channel = "dev"
	} else if dockerJson.Push_data.Tag == "stagging-" {
		channel = "stagging"
	} else if dockerJson.Push_data.Tag == "prod-" {
		channel = "prod"
	}
	fulltag := dockerJson.Repository.Repo_name + ":" + dockerJson.Push_data.Tag
	internal.GetLogger().Debug("channel: " + channel + ", fulltag: " + fulltag)

	if channel == "" {
		return
	}

	//publish
	// err = publishToNamespaceTag(channel, dockerJson.Repository.Repo_name, dockerJson.Push_data.Tag)
	// if err != nil {
	// 	internal.GetLogger().Error(err.Error())
	// }

	http.Error(w, http.StatusText(http.StatusNotFound), http.StatusNotFound)

})

func publishToConfigmap_label(cfgmapName string, imgRepoName string, imgTag string) error {
	cfgmap, err := internal.Cfg.K8ss_local.ClientSet.CoreV1().ConfigMaps(internal.Cfg.PodNS).Get(context.Background(), cfgmapName, metav1.GetOptions{})
	if err != nil {
		internal.GetLogger().Error(err.Error())
	}
	cfgmap.Labels[imgRepoName] = imgTag
	_, err = internal.Cfg.K8ss_local.ClientSet.CoreV1().ConfigMaps(internal.Cfg.PodNS).Update(context.Background(), cfgmap, metav1.UpdateOptions{})
	return err
}

// func publishToConfigmap_data(cfgmapName string, channel string, imgRepoName string, imgTag string) error {
// 	cfgmap, err := internal.Cfg.K8ss_local.ClientSet.CoreV1().ConfigMaps(internal.Cfg.PodNS).Get(context.Background(), cfgmapName, metav1.GetOptions{})
// 	if err != nil {
// 		internal.GetLogger().Error(err.Error())
// 	}
// 	cfgkey := channel + "." + strings.Replace(imgRepoName, "/", "_", -1)
// 	cfgmap.Data[cfgkey] = imgTag
// 	_, err = internal.Cfg.K8ss_local.ClientSet.CoreV1().ConfigMaps(internal.Cfg.PodNS).Update(context.Background(), cfgmap, metav1.UpdateOptions{})
// 	return err
// }

// func processNsList(nsList *corev1.NamespaceList, channel string, repoName string, tag string) {
// 	for _, item := range nsList.Items {
// 		nsName := item.Name
// 		internal.GetLogger().Debug("nsName: " + nsName)

// 		dClient := internal.Cfg.K8ss_local.ClientSet.AppsV1().Deployments(nsName)

// 		if strings.HasPrefix(nsName, "hc-") && item.Labels["channel"] == channel {
// 			dList, err := dClient.List(context.Background(), metav1.ListOptions{})
// 			if err != nil {
// 				internal.GetLogger().Panic(err.Error())
// 			}
// 			for _, d := range dList.Items {
// 				for i, c := range d.Spec.Template.Spec.Containers {
// 					internal.GetLogger().Debug("c.Image: " + c.Image + ", repoName: " + repoName)

// 					if strings.Split(c.Image, ":")[0] == repoName {
// 						d.Spec.Template.Spec.Containers[i].Image = repoName + ":" + tag
// 						_, err := dClient.Update(context.Background(), &d, metav1.UpdateOptions{})
// 						if err != nil {
// 							internal.GetLogger().Panic(err.Error())
// 						}
// 					}
// 				}
// 			}
// 		}
// 	}
// }

func prettyPrintJson(jsonBytes []byte) string {
	d := json.NewDecoder(bytes.NewBuffer(jsonBytes))
	var m map[string]interface{}
	_ = d.Decode(&m)
	b, _ := json.MarshalIndent(m, "", "  ")
	return string(b)
}

var TurkeyGitops = http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
	if r.URL.Path != "/webhooks/turkeygitops" || r.Method != "POST" {
		http.Error(w, http.StatusText(http.StatusNotFound), http.StatusNotFound)
		return
	}

	internal.GetLogger().Sugar().Debugf("dump headers: %v", r.Header)
	rBodyBytes, _ := ioutil.ReadAll(r.Body)
	internal.GetLogger().Debug(prettyPrintJson(rBodyBytes))

})
