package handlers

import (
	"bytes"
	"context"
	"crypto/tls"
	"encoding/json"
	"errors"
	"fmt"
	"io/ioutil"
	"main/internal"
	"math/rand"
	"mime/multipart"
	"net/http"
	"os"
	"strconv"
	"strings"
	"time"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

var Ita_admin_info = http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
	resp := map[string]interface{}{
		"ssh_totp_qr_data":     "N/A",
		"ses_max_24_hour_send": 99999,
		"using_ses":            true,
		"worker_domain":        "N/A",
		"assets_domain":        "N/A",
		"server_domain":        internal.Cfg.Domain,
		"provider":             "N/A",
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(resp)

})

var Ita_cfg_ret_ps = http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
	resp := map[string]interface{}{}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(resp)

})

var HC_launch_fallback = http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
	if r.URL.Path != "/hc_launch_fallback" || r.Method != "GET" {
		http.Error(w, http.StatusText(http.StatusNotFound), http.StatusNotFound)
		return
	}

	internal.Logger.Sugar().Debugf("dump headers: %v", r.Header)

	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	html := `
	<h1> your hubs services are warming up, <br>
	it went cold because it's a free tier<br>
	or the pod somehow dead... <br>
	anyway check back in 30-ish-seconds </h1> <br>
	this is still wip ... <b>/hc_launch_fallback</> ... <br>
	todo: <br>
	need a better looking page here
	`
	fmt.Fprint(w, html)

})

var trc_ok_RespMsg = `
<h1> your hubs infra's starting up </h1>
disclaimer: this page is still wip ... <b>/trc</b> ... <br>
todo(internal only): <br>
1. a better looking page here <br>
`
var trc_cd_RespMsg = `
<h1> too soon </h1>
disclaimer: this page is still wip ... <b>/trc</b> ... <br>
todo(internal only): <br>
1. a better looking page here <br>
`
var trc_err_RespMsg = `
<h1> your hubs infra's dead ... <br> but don't worry because some engineers on our end's getting a pagerduty for it </h1>
disclaimer: this page is still wip ... <b>/trc</b> ... <br>
todo(internal only): <br>
1. a better looking page here <br>
`

// todo: put strict rate limit on this endpoint and add caching to deflect/protect against ddos
var TurkeyReturnCenter = http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
	// if r.URL.Path != "/global_404_fallback" || r.Method != "GET" {
	// 	http.Error(w, http.StatusText(http.StatusNotFound), http.StatusNotFound)
	// 	return
	// }
	goods := r.URL.Query().Get("goods")
	if !strings.HasSuffix(goods, internal.Cfg.Domain) {
		internal.Logger.Sugar().Debugf("TurkeyReturnCenter bounce / !strings.HasSuffix(goods (%v), internal.Cfg.Domain(%v)) ", goods, internal.Cfg.Domain)
		w.Header().Set("turkey", "?")
		http.Error(w, http.StatusText(http.StatusNotFound), http.StatusNotFound)
		return
	}

	internal.Logger.Sugar().Debugf("dump headers: %v", r.Header)

	subdomain := strings.Split(goods, ".")[0]

	nsName := internal.HC_NS_MAN.GetNsName(subdomain)
	// not requesting a hubs cloud namespace == bounce
	if nsName == "" {
		// internal.Logger.Debug("TurkeyReturnCenter bounce / internal.HC_NS_MAN.GetNsName doesn't have a nsName for subdomain: " + subdomain)
		w.Header().Set("turkey", "??")
		http.Error(w, http.StatusText(http.StatusNotFound), http.StatusNotFound)
		return
	}

	w.WriteHeader(http.StatusServiceUnavailable)

	notes := internal.HC_NS_MAN.Get(nsName)
	if notes.Lastchecked.IsZero() {
		w.Header().Set("turkey", "???")
		internal.Logger.Error("did not find expected nsName: <" + nsName + "> for subdomain: <" + subdomain + ">")
		http.Error(w, http.StatusText(http.StatusNotFound), http.StatusNotFound)
		return
	}
	// high frequency pokes == bounce
	coolDown := 15 * time.Minute
	waitReq := coolDown - time.Since(notes.Lastchecked)
	if waitReq > 0 {
		internal.Logger.Debug("on coolDown bounc for: " + subdomain)
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		w.Header().Set(
			"Retry-After",
			fmt.Sprintf("%f", waitReq.Seconds()),
		)

		fmt.Fprint(w, fmt.Sprintf(`%v <br> -->  <a href=https://%v>%v</a> (try again in %v)`, trc_cd_RespMsg, goods, goods, waitReq.String()))
		return
	}

	//todo: check if Labeled with status=paused, otherwise it's probably an error/exception because the request should be catched by higher priority ingress rules inside hc-namespace
	//todo: check tiers for scaling configs
	//todo: test HPA (horizontal pod autoscaler)'s min settings instead of

	//just scale it back up to 1 for now
	go wakeupHcNs(nsName)
	internal.HC_NS_MAN.Set(nsName, internal.HcNsNotes{Lastchecked: time.Now()})

	internal.Logger.Debug("wakeupHcNs launched for nsName: " + nsName)

	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	fmt.Fprint(w, fmt.Sprintf(`%v <br> -->  <a href=https://%v>%v</a>`, trc_ok_RespMsg, goods, goods))
})

func wakeupHcNs(ns string) {

	//todo: get and handle tier configs

	//scale things back up in this namespace
	ds, err := internal.Cfg.K8ss_local.ClientSet.AppsV1().Deployments(ns).List(context.Background(), metav1.ListOptions{})
	if err != nil {
		internal.Logger.Error("wakeupHcNs - failed to list deployments: " + err.Error())
	}
	if len(ds.Items) < 1 {
		internal.Logger.Error("wakeupHcNs - deployment list is empty for namespace: " + ns)
	}

	scaleUpTo := 1
	for _, d := range ds.Items {
		d.Spec.Replicas = pointerOfInt32(scaleUpTo)
		_, err := internal.Cfg.K8ss_local.ClientSet.AppsV1().Deployments(ns).Update(context.Background(), &d, metav1.UpdateOptions{})
		if err != nil {
			internal.Logger.Error("wakeupHcNs -- failed to scale <ns: " + ns + ", deployment: " + d.Name + "> back up: " + err.Error())
		}
	}

}

//

type ret_asset struct {
	Text_id     string `json:"_text_id"`
	Slug        string `json:"slug"`
	Name        string `json:"name"`
	Inserted_at string `json:"inserted_at"`
	Updated_at  string `json:"updated_at"`
	//scenes
	Model_owned_file_id      json.Number `json:"model_owned_file_id"`
	Scene_owned_file_id      json.Number `json:"scene_owned_file_id"`
	Screenshot_owned_file_id json.Number `json:"screenshot_owned_file_id"`
	//avatars
	Gltf_owned_file_id         json.Number `json:"gltf_owned_file_id"`
	Bin_owned_file_id          json.Number `json:"bin_owned_file_id"`
	Thumbnail_owned_file_id    json.Number `json:"thumbnail_owned_file_id"`
	Base_map_owned_file_id     json.Number `json:"base_map_owned_file_id"`
	Emissive_map_owned_file_id json.Number `json:"emissive_map_owned_file_id"`
	Normal_map_owned_file_id   json.Number `json:"normal_map_owned_file_id"`
	Orm_map_owned_file_id      json.Number `json:"orm_map_owned_file_id"`
}

func ret_avatar_post_import(getReqBody []byte, subdomain, domain, token string) error {

	assets := []ret_asset{}
	json.Unmarshal(getReqBody, &assets)
	if len(assets) < 1 {
		return errors.New("(@ret_avatar_post_import)bad getReqBody: " + string(getReqBody))
	}
	asset := assets[0]
	sid := "z" + fmt.Sprintf("%d", rand.Intn(999999))
	listReqBody := []byte(`
	{
		"avatar_listing_sid": "` + sid + `",
		"avatar_id": "` + asset.Text_id + `",
		"slug": "` + asset.Slug + `",
		"name": "` + asset.Name + `",
		"description": null,
		"attributions": {},							
		"tags": {
			"tags": [
				"default",
				"featured"
			]
		},
		"parent_avatar_listing_id": null,
		"gltf_owned_file_id": ` + quotedString_or_nullForEmpty(asset.Gltf_owned_file_id.String()) + `,
		"bin_owned_file_id": ` + quotedString_or_nullForEmpty(asset.Bin_owned_file_id.String()) + `,
		"thumbnail_owned_file_id": ` + quotedString_or_nullForEmpty(asset.Thumbnail_owned_file_id.String()) + `,
		"base_map_owned_file_id": ` + quotedString_or_nullForEmpty(asset.Base_map_owned_file_id.String()) + `,
		"emissive_map_owned_file_id": ` + quotedString_or_nullForEmpty(asset.Emissive_map_owned_file_id.String()) + `,
		"normal_map_owned_file_id": ` + quotedString_or_nullForEmpty(asset.Normal_map_owned_file_id.String()) + `,
		"orm_map_owned_file_id": ` + quotedString_or_nullForEmpty(asset.Orm_map_owned_file_id.String()) + `,
		"order": 10000,
		"state": "active",
		"inserted_at": "` + asset.Inserted_at + `",
		"updated_at": "` + asset.Updated_at + `"
	}
	`)

	internal.Logger.Sugar().Debugf("listReqBody: %v", string(listReqBody))

	listReq, _ := http.NewRequest(
		"POST",
		"https://"+subdomain+"."+domain+"/api/postgrest/avatar_listings",
		bytes.NewBuffer(listReqBody),
	)
	listReq.Header.Add("content-type", "application/json")
	listReq.Header.Add("authorization", "bearer "+string(token))
	_httpClient := &http.Client{
		Timeout:   10 * time.Second,
		Transport: &http.Transport{TLSClientConfig: &tls.Config{InsecureSkipVerify: true}},
	}
	resp, err := _httpClient.Do(listReq)
	if err != nil {
		return err
	}
	internal.Logger.Sugar().Debugf(" listReq resp: %v", resp)
	respBody, _ := ioutil.ReadAll(resp.Body)
	internal.Logger.Sugar().Debugf(" listReq respBody: %v", string(respBody))

	return nil
}

func ret_scene_post_import(getReqBody []byte, subdomain, domain, token string) error {

	assets := []ret_asset{}
	json.Unmarshal(getReqBody, &assets)

	if len(assets) < 1 {
		return errors.New("(@ret_scene_post_import)bad getReqBody: " + string(getReqBody))
	}
	asset := assets[0]
	sid := "z" + fmt.Sprintf("%d", rand.Intn(999999))
	listReqBody := []byte(`
	{
		"scene_listing_sid": "` + sid + `",
		"scene_id": "` + asset.Text_id + `",
		"slug": "` + asset.Slug + `",
		"name": "` + asset.Name + `",
		"description": null,
		"attributions": {
			"content": [],
			"creator": ""
		},
		"tags": {
			"tags": [
				"default",
				"featured"
			]
		},
		"model_owned_file_id": ` + quotedString_or_nullForEmpty(asset.Model_owned_file_id.String()) + `,
		"scene_owned_file_id": ` + quotedString_or_nullForEmpty(asset.Scene_owned_file_id.String()) + `,
		"screenshot_owned_file_id": ` + quotedString_or_nullForEmpty(asset.Screenshot_owned_file_id.String()) + `,
		"order": 10000,
		"state": "active",
		"inserted_at": "` + asset.Inserted_at + `",
		"updated_at": "` + asset.Updated_at + `"
	}
	`)

	internal.Logger.Sugar().Debugf("listReqBody: %v", string(listReqBody))
	listReq, _ := http.NewRequest(
		"POST",
		"https://"+subdomain+"."+domain+"/api/postgrest/scene_listings",
		bytes.NewBuffer(listReqBody),
	)
	listReq.Header.Add("content-type", "application/json")
	listReq.Header.Add("authorization", "bearer "+string(token))
	_httpClient := &http.Client{
		Timeout:   10 * time.Second,
		Transport: &http.Transport{TLSClientConfig: &tls.Config{InsecureSkipVerify: true}},
	}
	resp, err := _httpClient.Do(listReq)
	if err != nil {
		return err
	}
	internal.Logger.Sugar().Debugf(" listReq resp: %v", resp)
	respBody, _ := ioutil.ReadAll(resp.Body)
	internal.Logger.Sugar().Debugf(" listReq respBody: %v", string(respBody))

	return nil
}

func quotedString_or_nullForEmpty(in string) string {
	if in == "" {
		return `null`
	}
	return `"` + in + `"`
}

var LetsencryptAccountCollect = http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
	if r.URL.Path != "/letsencrypt-account-collect" || r.Method != "POST" {
		http.Error(w, http.StatusText(http.StatusNotFound), http.StatusNotFound)
		return
	}

	// letsencryptAcct := r.Header.Get("letsencrypt-account")
	rbodyBytes, _ := ioutil.ReadAll(r.Body)
	letsencryptAcct := string(rbodyBytes)
	cm, err := internal.Cfg.K8ss_local.ClientSet.CoreV1().ConfigMaps("turkey-services").Get(context.Background(), "letsencrypt-accounts", metav1.GetOptions{})
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		return
	}
	acctName := "acct-" + strconv.Itoa(len(cm.Data))
	cm.Data[acctName] = letsencryptAcct
	cm.ResourceVersion = ""
	_, err = internal.Cfg.K8ss_local.ClientSet.CoreV1().ConfigMaps("turkey-services").Update(context.Background(), cm, metav1.UpdateOptions{})
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	internal.Logger.Sugar().Debugf("collected letsencryptAcct: %v", letsencryptAcct)

	w.WriteHeader(http.StatusOK)
	fmt.Fprintf(w, "collected: "+acctName)
})

var Dump_HcNsTable = http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
	if r.URL.Path != "/dump_hcnstable" {
		http.Error(w, http.StatusText(http.StatusNotFound), http.StatusNotFound)
		return
	}

	fmt.Fprintf(w, fmt.Sprintf("%v", internal.HC_NS_MAN.Dump()))
})

func ret_upload_files(subdomain, domain string, files map[string]interface{}) (map[string]interface{}, error) {
	for k, _ := range files {
		respMap, err := ret_upload_file(subdomain, domain, k)
		if err != nil {
			// return nil, err
			files[k] = err
		}
		files[k] = respMap
	}
	return files, nil
}

func ret_upload_file(subdomain, domain, filePath string) (respMap map[string]interface{}, err error) {
	url := "https://" + subdomain + "." + domain + "/api/v1/media"

	// Create the multipart/form-data payload
	payload := &bytes.Buffer{}
	writer := multipart.NewWriter(payload)
	// Add the media file to the request
	file, err := os.Open(filePath)
	if err != nil {
		return nil, fmt.Errorf("os.Open(filePath): %v", err)
	}
	defer file.Close()

	part, err := writer.CreateFormFile("media", filePath)
	if err != nil {
		return nil, fmt.Errorf("writer.CreateFormFile: %v", err)
	}
	partBytes, err := ioutil.ReadAll(file)
	if err != nil {
		return nil, fmt.Errorf("ioutil.ReadAll(file): %v", err)
	}
	part.Write(partBytes)
	_ = writer.WriteField("type", "image/jpeg")
	_ = writer.WriteField("promotion_mode", "with_token")
	err = writer.Close()
	if err != nil {
		return nil, fmt.Errorf("writer.Close(): %v", err)
	}
	req, err := http.NewRequest("POST", url, payload)
	if err != nil {
		return nil, fmt.Errorf("http.NewRequest: %v", err)
	}
	req.Header.Set("Content-Type", writer.FormDataContentType())
	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("decoder.Decode(&respMap): %v", err)
	}
	defer resp.Body.Close()

	// bodyBytes, _ := io.ReadAll(resp.Body)
	// fmt.Println("bodyBytes", string(bodyBytes))

	decoder := json.NewDecoder(resp.Body)

	respMap = make(map[string]interface{})
	err = decoder.Decode(&respMap)
	if err != nil {
		return nil, fmt.Errorf("decoder.Decode(&respMap): %v", err)
	}
	// return respMap["origin"].(string), respMap["meta"].(map[string]interface{})["access_token"].(string), nil
	return respMap, nil
}

func ret_setDefaultTheme(token []byte, cfg HCcfg) error {
	//load default theme file
	themeBytes, err := ioutil.ReadFile("./_files/hc_assets/theme.json")
	if err != nil {
		return err
	}

	//upload default logos
	logo_files := map[string]interface{}{
		"./_files/hc_assets/HubLogo.svg":            nil,
		"./_files/hc_assets/HubLogoForDarkMode.svg": nil,
		"./_files/hc_assets/Favicon.ico":            nil,
		"./_files/hc_assets/HomePageImage.jpg":      nil,
		"./_files/hc_assets/CompanyLogo.png":        nil,
		"./_files/hc_assets/ShortcutIcon.png":       nil,
		"./_files/hc_assets/SocialMediaCard.jpg":    nil,
	}
	logo_files, err = ret_upload_files(cfg.Subdomain, cfg.HubDomain, logo_files)
	if err != nil {
		return err
	}
	internal.Logger.Sugar().Debugf("logo_files: %v", logo_files)

	//post app_configs
	appConfigsJsonBytes, err := json.Marshal(map[string]interface{}{
		"theme": map[string]string{
			"themes": string(themeBytes)},
		"images": map[string]interface{}{
			"logo_dark":       logo_files["./_files/hc_assets/HubLogoForDarkMode.svg"],
			"logo":            logo_files["./_files/hc_assets/HubLogo.svg"],
			"home_background": logo_files["./_files/hc_assets/HomePageImage.jpg"],
			"favicon":         logo_files["./_files/hc_assets/Favicon.ico"],
			"app_thumbnail":   logo_files["./_files/hc_assets/SocialMediaCard.jpg"],
			"app_icon":        logo_files["./_files/hc_assets/ShortcutIcon.png"],
			"company_logo":    logo_files["./_files/hc_assets/CompanyLogo.png"],
		},
	})
	if err != nil {
		return err
	}
	app_configs_req, err := http.NewRequest("POST", "https://"+cfg.Subdomain+"."+cfg.HubDomain+"/api/v1/app_configs", bytes.NewBuffer(appConfigsJsonBytes))
	if err != nil {
		return err
	}
	app_configs_req.Header.Set("Content-Type", "application/json")
	app_configs_req.Header.Add("authorization", "bearer "+string(token))
	client := &http.Client{}
	app_configs_resp, err := client.Do(app_configs_req)
	if err != nil {
		return err
	}
	internal.Logger.Sugar().Debugf("app_configs_resp: ", app_configs_resp)
	return nil
}

func ret_getAdminToken(cfg HCcfg) ([]byte, error) {

	_httpClient := &http.Client{
		Timeout:   5 * time.Second,
		Transport: &http.Transport{TLSClientConfig: &tls.Config{InsecureSkipVerify: true}},
	}

	tokenReq, _ := http.NewRequest(
		"POST",
		"http://ret.hc-"+cfg.HubId+":4001/api-internal/v1/make_auth_token_for_email",
		bytes.NewBuffer([]byte(`{"email":"`+cfg.UserEmail+`"}`)),
	)
	tokenReq.Header.Add("content-type", "application/json")
	tokenReq.Header.Add("x-ret-dashboard-access-key", internal.Cfg.DASHBOARD_ACCESS_KEY)
	resp, _, err := internal.RetryHttpReq(_httpClient, tokenReq, 15*time.Second)
	if err != nil {
		return nil, err
	}
	token, _ := ioutil.ReadAll(resp.Body)
	internal.Logger.Sugar().Debugf("admin-token: %v, hubId: %v", string(token), cfg.HubId)

	return token, nil

}

func hc_updateTier(cfg HCcfg) error {

	nsName := "hc-" + cfg.HubId

	// flush envVar to all containers
	ds, err := internal.Cfg.K8ss_local.ClientSet.AppsV1().Deployments(nsName).List(context.Background(), metav1.ListOptions{})
	if err != nil {
		return err
	}

	for _, d := range ds.Items {
		for _, c := range d.Spec.Template.Spec.Containers {
			gotTIER := false
			for idx, envVar := range c.Env {
				if envVar.Name == "TIER" {
					c.Env[idx].Value = cfg.Tier
					gotTIER = true
					break
				}
			}
			if !gotTIER {
				c.Env = append(c.Env, corev1.EnvVar{Name: "TIER", Value: cfg.Tier})
			}
			gotTurkeyCfg_tier := false
			for idx, envVar := range c.Env {
				if envVar.Name == "turkeyCfg_tier" {
					c.Env[idx].Value = cfg.Tier
					gotTurkeyCfg_tier = true
					break
				}
			}
			if !gotTurkeyCfg_tier {
				c.Env = append(c.Env, corev1.EnvVar{Name: "turkeyCfg_tier", Value: cfg.Tier})
			}
		}
		_, err = internal.Cfg.K8ss_local.ClientSet.AppsV1().Deployments(nsName).Update(context.Background(), &d, metav1.UpdateOptions{})
		if err != nil {
			return err
		}
	}

	//reset theme for p0/free tier
	if cfg.Tier == "p0" || cfg.Tier == "free" {
		token, err := ret_getAdminToken(cfg)
		if err != nil {
			return err
		}
		ret_setDefaultTheme(token, cfg)
	}

	return nil
}
