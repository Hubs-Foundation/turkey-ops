package handlers

import (
	"bufio"
	"bytes"
	"context"
	"crypto/tls"
	"crypto/x509"
	"encoding/base64"
	"encoding/json"
	"encoding/pem"
	"errors"
	"fmt"
	"io/ioutil"
	"os"
	"regexp"

	"net/http"
	"net/url"
	"strings"
	"time"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"

	"main/internal"
)

type hcCfg struct {

	//required input
	UserEmail string `json:"useremail"`

	//required, but with fallbacks
	AccountId    string `json:"account_id"`    // turkey account id, fallback to random string produced by useremail seeded rnd func
	HubId        string `json:"hub_id"`        // id of the hub instance, also used to name the hub's namespace, fallback to  random string produced by AccountId seeded rnd func
	Tier         string `json:"tier"`          // fallback to free
	CcuLimit     string `json:"ccu_limit"`     // fallback to 20
	StorageLimit string `json:"storage_limit"` // fallback to 0.5
	Subdomain    string `json:"subdomain"`     // fallback to HubId
	NodePool     string `json:"nodepool"`      // default == spot

	//optional inputs
	Options string `json:"options"` //additional options, debug purpose only, underscore(_)prefixed -- ie. "_nfs"

	//inherited from turkey cluster -- aka the values are here already, in internal.Cfg
	Domain               string `json:"domain"`
	HubDomain            string `json:"hubdomain"`
	DBname               string `json:"dbname"`
	DBpass               string `json:"dbpass"`
	FilestoreIP          string
	FilestorePath        string
	PermsKey             string `json:"permskey"`
	SmtpServer           string `json:"smtpserver"`
	SmtpPort             string `json:"smtpport"`
	SmtpUser             string `json:"smtpuser"`
	SmtpPass             string `json:"smtppass"`
	GCP_SA_KEY_b64       string `json:"gcp_sa_key_b64"`
	ItaChan              string `json:"itachan"`            //build channel for ita to track
	GCP_SA_HMAC_KEY      string `json:"GCP_SA_HMAC_KEY"`    //https://cloud.google.com/storage/docs/authentication/hmackeys, ie.GOOG1EGPHPZU7F3GUTJCVQWLTYCY747EUAVHHEHQBN4WXSMPXJU4488888888
	GCP_SA_HMAC_SECRET   string `json:"GCP_SA_HMAC_SECRET"` //https://cloud.google.com/storage/docs/authentication/hmackeys, ie.0EWCp6g4j+MXn32RzOZ8eugSS5c0fydT88888888
	DASHBOARD_ACCESS_KEY string
	SKETCHFAB_API_KEY    string
	//generated on the fly
	JWK          string `json:"jwk"` // encoded from PermsKey.public
	GuardianKey  string `json:"guardiankey"`
	PhxKey       string `json:"phxkey"`
	HEADER_AUTHZ string `json:"headerauthz"`
	NODE_COOKIE  string `json:"NODE_COOKIE"`

	Img_ret           string
	Img_postgrest     string
	Img_ita           string
	Img_gcsfuse       string
	Img_hubs          string
	Img_spoke         string
	Img_speelycaptor  string
	Img_photomnemonic string
	// Img_nearspark string
	// Img_ytdl string
}

var HC_instance = http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {

	if r.URL.Path != "/hc_instance" {
		http.Error(w, http.StatusText(http.StatusNotFound), http.StatusNotFound)
		return
	}

	switch r.Method {
	case "POST":
		hc_create(w, r)
	case "GET":
		hc_get(w, r)
	case "DELETE":
		hc_delete(w, r)
	case "PATCH":
		cfg, err := getHcCfg(r)
		if err != nil {
			http.Error(w, http.StatusText(http.StatusBadRequest)+":"+err.Error(), http.StatusBadRequest)
			return
		}
		//
		status := r.URL.Query().Get("status")
		if status == "down" || status == "up" {
			err := hc_switch(cfg.HubId, status)
			if err != nil {
				w.WriteHeader(http.StatusInternalServerError)
				json.NewEncoder(w).Encode(map[string]interface{}{
					"error":  err.Error(),
					"hub_id": cfg.HubId,
				})
			}
			json.NewEncoder(w).Encode(map[string]interface{}{
				"msg":        "status updated",
				"new_status": status,
				"hub_id":     cfg.HubId,
			})
			return
		}

		if cfg.Subdomain != "" {
			// err := hc_patch_subdomain(cfg.HubId, cfg.Subdomain)
			// if err != nil {
			// 	w.WriteHeader(http.StatusInternalServerError)
			// 	json.NewEncoder(w).Encode(map[string]interface{}{
			// 		"error":  err.Error(),
			// 		"hub_id": cfg.HubId,
			// 	})
			// 	return
			// }
			go func() {
				err := hc_patch_subdomain(cfg.HubId, cfg.Subdomain)
				if err != nil {
					internal.Logger.Error("hc_patch_subdomain FAILED: " + err.Error())
				}
			}()
			json.NewEncoder(w).Encode(map[string]interface{}{
				"msg":           "subdomain updated",
				"new_subdomain": cfg.Subdomain,
				"hub_id":        cfg.HubId,
			})
			return
		}
		json.NewEncoder(w).Encode(map[string]interface{}{
			"msg":    "",
			"hub_id": cfg.HubId,
		})
	}

})

func hc_create(w http.ResponseWriter, r *http.Request) {

	// sess := internal.GetSession(r.Cookie)

	// #1 prepare configs
	hcCfg, err := makeHcCfg(r)
	if err != nil {
		internal.Logger.Error("bad hcCfg: " + err.Error())
		http.Error(w, http.StatusText(http.StatusBadRequest), http.StatusBadRequest)
		return
	}

	// #2 render turkey-k8s-chart by apply cfg to hc.yam
	fileOption := "_gfs"      //default
	if hcCfg.Tier == "free" { //default for free tier
		fileOption = "_nfs"
	}
	//override
	if hcCfg.Options != "" {
		fileOption = hcCfg.Options
	}

	if fileOption == "_fs" || fileOption == "_gfs" { //create folder for hub-id (hc-<hub_id>) in turkeyfs
		os.MkdirAll("/turkeyfs/hc-"+hcCfg.HubId, 0600)
	}

	internal.Logger.Debug(" >>>>>> selected option: " + fileOption)

	yamBytes, err := ioutil.ReadFile("./_files/yams/ns_hc" + fileOption + ".yam")
	if err != nil {
		internal.Logger.Error("failed to get ns_hc yam file because: " + err.Error())
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(map[string]interface{}{
			"result": "error @ getting k8s chart file: " + err.Error(),
			"hub_id": hcCfg.HubId,
		})
		return
	}

	renderedYamls, err := internal.K8s_render_yams([]string{string(yamBytes)}, hcCfg)
	if err != nil {
		internal.Logger.Error("failed to render ns_hc yam file because: " + err.Error())
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(map[string]interface{}{
			"result": "error @ rendering k8s chart file: " + err.Error(),
			"hub_id": hcCfg.HubId,
		})
		return
	}
	k8sChartYaml := renderedYamls[0]

	// #3 pre-deployment checks
	// // result := "created."
	// // is subdomain available?
	// nsList, _ := internal.Cfg.K8ss_local.ClientSet.CoreV1().Namespaces().List(context.Background(),
	// 	metav1.ListOptions{LabelSelector: "Subdomain=" + hcCfg.Subdomain})
	// if len(nsList.Items) != 0 {
	// 	// if strings.HasSuffix(r.Header.Get("X-Forwarded-UserEmail"), "@mozilla.com") {
	// 	// 	result += "[warning] subdomain already used by hub_id " + nsList.Items[0].Name + ", but was overridden for @mozilla.com user"
	// 	// } else {
	// 	internal.Logger.Error("error @ k8s pre-deployment checks: subdomain already exists: " + hcCfg.Subdomain)
	// 	w.WriteHeader(http.StatusBadRequest)
	// 	json.NewEncoder(w).Encode(map[string]interface{}{
	// 		"result": "subdomain already exists",
	// 		"hub_id": hcCfg.HubId,
	// 	})
	// 	return
	// 	// }
	// }

	// #4 kubectl apply -f <file.yaml> --server-side --field-manager "turkey-userid-<cfg.UserId>"
	internal.Logger.Debug("&#128640; --- deployment started for: " + hcCfg.HubId)
	err = internal.Ssa_k8sChartYaml(hcCfg.AccountId, k8sChartYaml, internal.Cfg.K8ss_local.Cfg)
	if err != nil {
		internal.Logger.Error("error @ k8s deploy: " + err.Error())
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(map[string]interface{}{
			"result": "error @ k8s deploy: " + err.Error(),
			"hub_id": hcCfg.HubId,
		})
		return
	}

	// // quality of life improvement for /console people
	// skipadminLink := "https://" + hcCfg.Subdomain + "." + hcCfg.Domain + "?skipadmin"
	// internal.Logger.Debug("&#128640; --- deployment completed for: <a href=\"" +
	// 	skipadminLink + "\" target=\"_blank\"><b>&#128279;" + hcCfg.AccountId + ":" + hcCfg.Subdomain + "</b></a>")
	// internal.Logger.Debug("&#128231; --- admin email: " + hcCfg.UserEmail)

	// #5 create db
	_, err = internal.PgxPool.Exec(context.Background(), "create database \""+hcCfg.DBname+"\"")
	if err != nil {
		if strings.Contains(err.Error(), "already exists (SQLSTATE 42P04)") {
			internal.Logger.Debug("db already exists: " + hcCfg.DBname)
		} else {
			internal.Logger.Error("error @ create hub db: " + err.Error())
			json.NewEncoder(w).Encode(map[string]interface{}{
				"result": "error @ create hub db: " + err.Error(),
				"hub_id": hcCfg.HubId,
			})
			return
		}
	}
	internal.Logger.Debug("&#128024; --- db created: " + hcCfg.DBname)

	go func() {
		err := sync_load_assets(hcCfg)
		if err != nil {
			internal.Logger.Error("sync_load_assets FAILED: " + err.Error())
		}
	}()

	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(map[string]interface{}{
		"result":        "done",
		"useremail":     hcCfg.UserEmail,
		"hub_id":        hcCfg.HubId,
		"subdomain":     hcCfg.Subdomain,
		"tier":          hcCfg.Tier,
		"ccu_limit":     hcCfg.CcuLimit,
		"storage_limit": hcCfg.StorageLimit,
	})
}

func sync_load_assets(cfg hcCfg) error {

	_httpClient := &http.Client{
		Timeout:   10 * time.Second,
		Transport: &http.Transport{TLSClientConfig: &tls.Config{InsecureSkipVerify: true}},
	}
	//wait for ret
	retReq, _ := http.NewRequest("GET", "https://"+cfg.Subdomain+"."+cfg.Domain+"/health", nil)

	_, took, err := internal.RetryHttpReq(_httpClient, retReq, 5*time.Minute)
	if err != nil {
		return err
	}
	internal.Logger.Sugar().Debugf("tReady: %v, hubId: %v", took, cfg.HubId)

	//get admin auth token
	tokenReq, _ := http.NewRequest(
		"POST",
		"https://ret.hc-"+cfg.HubId+":4000/api-internal/v1/make_auth_token_for_email",
		bytes.NewBuffer([]byte(`{"email":"`+cfg.UserEmail+`"}`)),
	)
	tokenReq.Header.Add("content-type", "application/json")
	tokenReq.Header.Add("x-ret-dashboard-access-key", internal.Cfg.DASHBOARD_ACCESS_KEY)
	resp, _, err := internal.RetryHttpReq(_httpClient, tokenReq, 1*time.Minute)
	if err != nil {
		return err
	}
	token, _ := ioutil.ReadAll(resp.Body)
	internal.Logger.Sugar().Debugf("admin-token: %v, hubId: %v", string(token), cfg.HubId)

	//load asset
	assetPackUrl := "https://raw.githubusercontent.com/mozilla/hubs-cloud/master/asset-packs/event.pack"
	resp, err = http.Get(assetPackUrl)
	if err != nil {
		return err
	}
	s := bufio.NewScanner(resp.Body)
	for s.Scan() {
		line := s.Text()
		url, err := url.Parse(strings.Trim(line, " "))
		if err != nil {
			return err
		}
		err = ret_load_asset(url, cfg.HubId, string(token))
		if err != nil {
			internal.Logger.Error(fmt.Sprintf("failed to load asset: %v, error: %v", url, err))
		}
	}
	return nil
}

func ret_load_asset(url *url.URL, hubId string, token string) error {
	pathArr := strings.Split(url.Path, "/")
	if len(pathArr) != 3 {
		return fmt.Errorf("unsupported url: %v", url)
	}
	kind_s := pathArr[1]
	kind := kind_s[:len(kind_s)-1]
	id := pathArr[2]
	if kind_s != "avatars" && kind_s != "scenes" {
		return fmt.Errorf("unsupported url: %v", url)
	}

	//import
	assetUrl := "https://" + url.Host + "/api/v1/" + kind_s + "/" + id
	loadReq, _ := http.NewRequest(
		"POST",
		"https://ret.hc-"+hubId+":4000/api/v1/"+kind_s,
		bytes.NewBuffer([]byte(`{"url":"`+assetUrl+`"}`)),
	)
	loadReq.Header.Add("content-type", "application/json")
	loadReq.Header.Add("authorization", "bearer "+string(token))

	_httpClient := &http.Client{
		Timeout:   10 * time.Second,
		Transport: &http.Transport{TLSClientConfig: &tls.Config{InsecureSkipVerify: true}},
	}
	resp, took, err := internal.RetryHttpReq(_httpClient, loadReq, 15*time.Second)
	if err != nil {
		return err
	}
	var importResp map[string][]map[string]interface{}
	rBody, _ := ioutil.ReadAll(resp.Body)
	json.Unmarshal(rBody, &importResp)
	newAssetId := importResp[kind_s][0][kind+"_id"].(string)
	internal.Logger.Sugar().Debugf("### import -- took: %v, loaded: %v, new_id: %v", took, assetUrl, newAssetId)

	//post import -- generate <kind>_listing_sid and post to <kind>_listings
	getReq, _ := http.NewRequest(
		"GET",
		"https://ret.hc-"+hubId+":4000/api/postgrest/"+kind_s+"?"+kind+"_sid=ilike.*"+newAssetId+"*",
		// bytes.NewBuffer([]byte(`{"url":"`+assetUrl+`"}`)),
		nil,
	)
	getReq.Header.Add("authorization", "bearer "+string(token))
	resp, err = _httpClient.Do(getReq)
	if err != nil {
		return err
	}
	getReqBody, _ := ioutil.ReadAll(resp.Body)

	if kind == "avatar" {
		err := ret_avatar_post_import(getReqBody, hubId, token)
		if err != nil {
			return err
		}
	} else if kind == "scene" {
		err := ret_scene_post_import(getReqBody, hubId, token)
		if err != nil {
			return err
		}
	} else {
		return errors.New("unexpected kind: " + kind)
	}

	return nil
}

func hc_get(w http.ResponseWriter, r *http.Request) {
	// sess := internal.GetSession(r.Cookie)

	cfg, err := getHcCfg(r)
	if err != nil {
		internal.Logger.Debug("bad hcCfg: " + err.Error())
		return
	}

	//getting k8s config
	internal.Logger.Debug("&#9989; ... using InClusterConfig")
	k8sCfg, err := rest.InClusterConfig()
	// }
	if k8sCfg == nil {
		internal.Logger.Debug("ERROR" + err.Error())
		internal.Logger.Error(err.Error())
		return
	}
	internal.Logger.Debug("&#129311; k8s.k8sCfg.Host == " + k8sCfg.Host)
	clientset, err := kubernetes.NewForConfig(k8sCfg)
	if err != nil {
		internal.Logger.Error(err.Error())
		return
	}
	//list ns
	nsList, err := clientset.CoreV1().Namespaces().List(context.TODO(),
		metav1.ListOptions{
			LabelSelector: "hub_id" + cfg.HubId,
		})
	if err != nil {
		internal.Logger.Error(err.Error())
		return
	}
	internal.Logger.Debug("GET --- user <" + cfg.AccountId + "> owns: ")
	for _, ns := range nsList.Items {
		internal.Logger.Debug("......ObjectMeta.GetName: " + ns.ObjectMeta.GetName())
		internal.Logger.Debug("......ObjectMeta.Labels.dump: " + fmt.Sprint(ns.ObjectMeta.Labels))
	}
}

func getHcCfg(r *http.Request) (hcCfg, error) {
	var cfg hcCfg
	rBodyBytes, err := ioutil.ReadAll(r.Body)
	if err != nil {
		internal.Logger.Error("ERROR @ reading r.body, error = " + err.Error())
		return cfg, err
	}
	err = json.Unmarshal(rBodyBytes, &cfg)
	if err != nil {
		internal.Logger.Error("bad hcCfg: " + string(rBodyBytes))
		return cfg, err
	}
	return cfg, nil
}

func makeHcCfg(r *http.Request) (hcCfg, error) {
	var cfg hcCfg
	rBodyBytes, err := ioutil.ReadAll(r.Body)
	if err != nil {
		internal.Logger.Error("ERROR @ reading r.body, error = " + err.Error())
		return cfg, err
	}
	err = json.Unmarshal(rBodyBytes, &cfg)
	if err != nil {
		internal.Logger.Error("bad hcCfg: " + string(rBodyBytes))
		return cfg, err
	}

	//userEmail supplied?
	if cfg.UserEmail == "" {
		cfg.UserEmail = r.Header.Get("X-Forwarded-UserEmail") //try fallback to authenticated useremail
	} //verify format?
	if cfg.UserEmail == "" { // can't create without email
		return cfg, errors.New("bad input, missing UserEmail or X-Forwarded-UserEmail")
		// cfg.UserEmail = "fooooo@barrrr.com"
	}

	// AccountId
	if cfg.AccountId == "" {
		cfg.AccountId = internal.PwdGen(8, int64(hash(cfg.UserEmail)), "")
		internal.Logger.Debug("AccountId unspecified, using: " + cfg.AccountId)
	}
	// HubId
	if cfg.HubId == "" {
		cfg.HubId = internal.PwdGen(6, int64(hash(cfg.AccountId+cfg.Subdomain)), "")
		internal.Logger.Debug("HubId unspecified, using: " + cfg.HubId)
	}
	//default Tier is free
	if cfg.Tier == "" {
		cfg.Tier = "free"
	}
	//default CcuLimit is 20
	if cfg.CcuLimit == "" {
		cfg.CcuLimit = "20"
	}
	//default StorageLimit is 0.5
	if cfg.StorageLimit == "" {
		cfg.StorageLimit = "0.5"
	}
	//default Subdomain is a string hashed from AccountId and time
	if cfg.Subdomain == "" {
		cfg.Subdomain = cfg.HubId
	}

	// cfg.DBname = "ret_" + strings.ReplaceAll(cfg.Subdomain, "-", "_")
	cfg.DBname = "ret_" + cfg.HubId

	//cluster wide private key for all reticulum authentications
	cfg.PermsKey = internal.Cfg.PermsKey
	if !strings.HasPrefix(cfg.PermsKey, `-----BEGIN RSA PRIVATE KEY-----`) {
		return cfg, errors.New("bad perms_key: " + cfg.PermsKey)
	}

	// if !strings.HasPrefix(cfg.PermsKey, `-----BEGIN RSA PRIVATE KEY-----\n`) {
	// 	fmt.Println(`cfg.PermsKey: replacing \n with \\n`)
	// 	cfg.PermsKey = strings.ReplaceAll(cfg.PermsKey, `\n`, `\\n`)
	// }

	//making cfg.JWK out of cfg.PermsKey ... pem.Decode needs "real" line breakers in the "pem byte array"
	perms_key_str := strings.Replace(cfg.PermsKey, `\\n`, "\n", -1)
	pb, _ := pem.Decode([]byte(perms_key_str))
	if pb == nil {
		internal.Logger.Error("failed to decode perms key")
		internal.Logger.Error("perms_key_str: " + perms_key_str)
		internal.Logger.Error(" cfg.PermsKey: " + cfg.PermsKey)
		return cfg, errors.New("failed to decode perms key")
	}
	perms_key, err := x509.ParsePKCS1PrivateKey(pb.Bytes)
	if err != nil {
		return cfg, err
	}
	//for postgrest to auth reticulum requests
	jwk, err := jwkEncode(perms_key.Public())
	if err != nil {
		return cfg, err
	}
	cfg.JWK = strings.ReplaceAll(jwk, `"`, `\"`)

	//inherit from cluster
	cfg.Domain = internal.Cfg.Domain
	cfg.HubDomain = internal.Cfg.HubDomain
	if cfg.HubDomain == "" {
		cfg.HubDomain = cfg.Domain
	}
	cfg.DBpass = internal.Cfg.DBpass
	cfg.FilestoreIP = internal.Cfg.FilestoreIP
	cfg.FilestorePath = internal.Cfg.FilestorePath
	cfg.SmtpServer = internal.Cfg.SmtpServer
	cfg.SmtpPort = internal.Cfg.SmtpPort
	cfg.SmtpUser = internal.Cfg.SmtpUser
	cfg.SmtpPass = internal.Cfg.SmtpPass
	cfg.GCP_SA_KEY_b64 = base64.StdEncoding.EncodeToString([]byte(os.Getenv("GCP_SA_KEY")))
	// cfg.ItaChan = internal.Cfg.Env
	if internal.Cfg.Env == "dev" {
		cfg.ItaChan = "dev"
	} else if internal.Cfg.Env == "staging" {
		cfg.ItaChan = "beta"
	} else {
		cfg.ItaChan = "stable"
	}
	cfg.GCP_SA_HMAC_KEY = internal.Cfg.GCP_SA_HMAC_KEY
	cfg.GCP_SA_HMAC_SECRET = internal.Cfg.GCP_SA_HMAC_SECRET
	cfg.DASHBOARD_ACCESS_KEY = internal.Cfg.DASHBOARD_ACCESS_KEY
	cfg.SKETCHFAB_API_KEY = internal.Cfg.SKETCHFAB_API_KEY
	//produce the rest
	if cfg.Tier == "free" || internal.Cfg.Env == "dev" {
		cfg.NodePool = "spot"
	} else {
		cfg.NodePool = "hub"
	}

	pwdSeed := int64(hash(cfg.Domain))
	cfg.GuardianKey = internal.PwdGen(15, pwdSeed, "G~")
	cfg.PhxKey = internal.PwdGen(15, pwdSeed, "P~")
	cfg.HEADER_AUTHZ = internal.PwdGen(15, pwdSeed, "H~")
	cfg.NODE_COOKIE = internal.PwdGen(15, pwdSeed, "N~")

	hubsbuildsCM, err := internal.Cfg.K8ss_local.ClientSet.CoreV1().ConfigMaps(internal.Cfg.PodNS).Get(context.Background(), "hubsbuilds-"+cfg.ItaChan, metav1.GetOptions{})
	if err != nil {
		internal.Logger.Error(err.Error())
	}
	imgRepo := internal.Cfg.ImgRepo
	cfg.Img_ret = "stable-latest"
	if hubsbuildsCM.Labels[imgRepo+"/ret"] != "" {
		cfg.Img_ret = hubsbuildsCM.Labels[imgRepo+"/ret"]
	}
	cfg.Img_postgrest = "stable-latest"
	if hubsbuildsCM.Labels[imgRepo+"/postgrest"] != "" {
		cfg.Img_postgrest = hubsbuildsCM.Labels[imgRepo+"/postgrest"]
	}
	cfg.Img_ita = "stable-latest"
	if hubsbuildsCM.Labels[imgRepo+"/ita"] != "" {
		cfg.Img_ita = hubsbuildsCM.Labels[imgRepo+"/ita"]
	}
	cfg.Img_gcsfuse = "stable-latest"
	if hubsbuildsCM.Labels[imgRepo+"/gcsfuse"] != "" {
		cfg.Img_gcsfuse = hubsbuildsCM.Labels[imgRepo+"/gcsfuse"]
	}
	cfg.Img_hubs = "stable-latest"
	if hubsbuildsCM.Labels[imgRepo+"/hubs"] != "" {
		cfg.Img_hubs = hubsbuildsCM.Labels[imgRepo+"/hubs"]
	}
	cfg.Img_spoke = "stable-latest"
	if hubsbuildsCM.Labels[imgRepo+"/spoke"] != "" {
		cfg.Img_spoke = hubsbuildsCM.Labels[imgRepo+"/spoke"]
	}
	cfg.Img_speelycaptor = "stable-latest"
	if hubsbuildsCM.Labels[imgRepo+"/speelycaptor"] != "" {
		cfg.Img_speelycaptor = hubsbuildsCM.Labels[imgRepo+"/speelycaptor"]
	}
	cfg.Img_photomnemonic = "stable-latest"
	if hubsbuildsCM.Labels[imgRepo+"/photomnemonic"] != "" {
		cfg.Img_photomnemonic = hubsbuildsCM.Labels[imgRepo+"/photomnemonic"]
	}

	return cfg, nil
}

func hc_delete(w http.ResponseWriter, r *http.Request) {
	// sess := internal.GetSession(r.Cookie)
	hcCfg, err := getHcCfg(r)
	if err != nil {
		internal.Logger.Error("bad hcCfg: " + err.Error())
		return
	}
	if hcCfg.HubId == "" {
		internal.Logger.Error("missing hcCfg.HubId")
		return
	}
	//mark the hc- namespace for the cleanup cronjob (todo)
	nsName := "hc-" + hcCfg.HubId
	err = internal.Cfg.K8ss_local.PatchNsAnnotation(nsName, "deleting", "true")
	if err != nil {
		internal.Logger.Error("failed @PatchNsAnnotation, err: " + err.Error())
		return
	}
	//remove ingress route
	err = internal.Cfg.K8ss_local.ClientSet.NetworkingV1().Ingresses(nsName).DeleteCollection(context.Background(), metav1.DeleteOptions{}, metav1.ListOptions{})
	if err != nil {
		internal.Logger.Error("failed @k8s.ingress.DeleteCollection, err: " + err.Error())
	}
	ttl := 1 * time.Minute
	wait := 5 * time.Second
	for il, _ := internal.Cfg.K8ss_local.ClientSet.NetworkingV1().Ingresses(nsName).List(context.Background(), metav1.ListOptions{}); len(il.Items) > 0; {
		time.Sleep(5 * time.Second)
		if ttl -= wait; ttl < 0 {
			internal.Logger.Error("timeout @ remove ingress")
			break
		}
	}

	go func() {
		internal.Logger.Debug("&#128024 deleting ns: " + nsName)
		// scale down the namespace before deletion to avoid pod/ns "stuck terminating"
		hc_switch(hcCfg.HubId, "down")
		err := internal.Cfg.K8ss_local.WaitForPodKill(nsName, 60*time.Minute, 1)
		if err != nil {
			internal.Logger.Error("error @WaitForPodKill: " + err.Error())
			return
		}
		//delete ns
		err = internal.Cfg.K8ss_local.ClientSet.CoreV1().Namespaces().Delete(context.TODO(),
			nsName,
			metav1.DeleteOptions{})
		if err != nil {
			internal.Logger.Error("delete ns failed: " + err.Error())
			return
		}
		internal.Logger.Debug("&#127754 deleted ns: " + nsName)
		//delete db
		hcCfg.DBname = "ret_" + hcCfg.HubId
		internal.Logger.Debug("&#128024 deleting db: " + hcCfg.DBname)
		force := true
		_, err = internal.PgxPool.Exec(context.Background(), "drop database "+hcCfg.DBname)
		if err != nil {
			if strings.Contains(err.Error(), "is being accessed by other users (SQLSTATE 55006)") && force {
				err = pg_kick_all(hcCfg)
				if err != nil {
					internal.Logger.Error(err.Error())
					return
				}
				_, err = internal.PgxPool.Exec(context.Background(), "drop database "+hcCfg.DBname)
			}
			if err != nil {
				internal.Logger.Error(err.Error())
				return
			}
		}
		internal.Logger.Debug("&#128024 deleted db: " + hcCfg.DBname)
		// delete GCS (if any)
		err = internal.Cfg.Gcps.GCS_DeleteObjects("turkeyfs", "hc-"+hcCfg.HubId+"."+internal.Cfg.Domain)
		if err != nil {
			internal.Logger.Error("delete ns failed: " + err.Error())
			return
		}
		// delete hc-<hub-id> folder on turkeyfs (if any)
		if len(hcCfg.HubId) < 1 {
			internal.Logger.Error("DANGER!!! empty HubId")
		} else {
			os.RemoveAll("/turkeyfs/hc-" + hcCfg.HubId)
		}
	}()

	//return
	w.WriteHeader(http.StatusAccepted)
	json.NewEncoder(w).Encode(map[string]interface{}{
		"result": "deleted",
		"hub_id": hcCfg.HubId,
	})
}

func pg_kick_all(cfg hcCfg) error {
	sqatterCount := -1
	tries := 0
	for sqatterCount != 0 && tries < 3 {
		squatters, _ := internal.PgxPool.Exec(context.Background(), `select usename,client_addr,state,query from pg_stat_activity where datname = '`+cfg.DBname+`'`)
		internal.Logger.Debug("WARNING: pg_kick_all: kicking <" + fmt.Sprint(squatters.RowsAffected()) + "> squatters from " + cfg.DBname)
		_, _ = internal.PgxPool.Exec(context.Background(), `REVOKE CONNECT ON DATABASE `+cfg.DBname+` FROM public`)
		_, _ = internal.PgxPool.Exec(context.Background(), `REVOKE CONNECT ON DATABASE `+cfg.DBname+` FROM `+internal.Cfg.DBuser)
		_, _ = internal.PgxPool.Exec(context.Background(), `SELECT pg_terminate_backend (pg_stat_activity.pid) FROM pg_stat_activity WHERE pg_stat_activity.datname = '`+cfg.DBname+`'`)
		time.Sleep(3 * time.Second)
		squatters, _ = internal.PgxPool.Exec(context.Background(), `select usename,client_addr,state,query from pg_stat_activity where datname = '`+cfg.DBname+`'`)
		sqatterCount = int(squatters.RowsAffected())
		tries++
	}
	if sqatterCount != 0 {
		return errors.New("ERROR: pg_kick_all: failed to kick <" + fmt.Sprint(sqatterCount) + "> squatter(s): ")
	}
	return nil
}

func hc_switch(HubId, status string) error {

	ns := "hc-" + HubId

	//acquire lock
	locker, err := internal.NewK8Locker(internal.NewK8sSvs_local().Cfg, ns)
	if err != nil {
		internal.Logger.Error("faild to acquire locker ... err: " + err.Error())
		return err
	}

	locker.Lock()

	Replicas := 0
	if status == "up" {
		// scale up to 1 and let hpa to manage scaling
		Replicas = 1
	}

	//deployments
	ds, err := internal.Cfg.K8ss_local.ClientSet.AppsV1().Deployments(ns).List(context.Background(), metav1.ListOptions{})
	if err != nil {
		internal.Logger.Error("hc_switch - failed to list deployments: " + err.Error())
		return err
	}

	for _, d := range ds.Items {
		d.Spec.Replicas = pointerOfInt32(Replicas)
		_, err := internal.Cfg.K8ss_local.ClientSet.AppsV1().Deployments(ns).Update(context.Background(), &d, metav1.UpdateOptions{})
		if err != nil {
			internal.Logger.Error("hc_switch -- failed to scale <ns: " + ns + ", deployment: " + d.Name + ">: " + err.Error())
			return err
		}
	}
	// //statefulset
	// sss, err := internal.Cfg.K8ss_local.ClientSet.AppsV1().StatefulSets(ns).List(context.Background(), metav1.ListOptions{})
	// if err != nil {
	// 	internal.Logger.Error("hc_switch - failed to list statefulsets: " + err.Error())
	// 	return err
	// }

	// for _, ss := range sss.Items {
	// 	ss.Spec.Replicas = pointerOfInt32(Replicas)
	// 	_, err := internal.Cfg.K8ss_local.ClientSet.AppsV1().StatefulSets(ns).Update(context.Background(), &ss, metav1.UpdateOptions{})
	// 	if err != nil {
	// 		internal.Logger.Error("hc_switch -- failed to scale <ns: " + ns + ", deployment: " + ss.Name + ">: " + err.Error())
	// 		return err
	// 	}
	// }

	// //waits
	// err = internal.Cfg.K8ss_local.WatiForDeployments(ns, 15*time.Minute)
	// if err != nil {
	// 	internal.Logger.Sugar().Errorf("failed @ WatiForDeployments: %v", err)
	// }

	locker.Unlock()
	return nil
}

func hc_patch_subdomain(HubId, Subdomain string) error {

	nsName := "hc-" + HubId

	// //waits
	// err := internal.Cfg.K8ss_local.WatiForDeployments(nsName, 15*time.Minute)
	// if err != nil {
	// 	internal.Logger.Sugar().Errorf("failed @ WatiForDeployments: %v", err)
	// }

	//update secret
	secret_configs, err := internal.Cfg.K8ss_local.ClientSet.CoreV1().Secrets(nsName).Get(context.Background(), "configs", metav1.GetOptions{})
	if err != nil {
		return err
	}

	oldSubdomain := string(secret_configs.Data["SUB_DOMAIN"])
	internal.Logger.Sugar().Debugf("[hc_patch_subdomain] %v => %v", oldSubdomain, Subdomain)

	secret_configs.StringData = map[string]string{"SUB_DOMAIN": Subdomain}
	_, err = internal.Cfg.K8ss_local.ClientSet.CoreV1().Secrets(nsName).Update(context.Background(), secret_configs, metav1.UpdateOptions{})
	if err != nil {
		return err
	}

	//update env vars in hubs and spoke deployments -- kind of hackish in order to support custom hubs and spoke config overrids, todo: refactor this
	for _, dName := range []string{"hubs", "spoke"} {
		d, err := internal.Cfg.K8ss_local.ClientSet.AppsV1().Deployments(nsName).Get(context.Background(), dName, metav1.GetOptions{})
		if err != nil {
			return err
		}
		for idx, envVar := range d.Spec.Template.Spec.Containers[0].Env {
			if !strings.Contains(envVar.Value, oldSubdomain) {
				continue
			}
			newSubdomain := d.Spec.Template.Spec.Containers[0].Env[idx].Value
			if strings.Contains(newSubdomain, `,`+oldSubdomain+`.`) {
				newSubdomain = strings.Replace(newSubdomain, `,`+oldSubdomain+`.`, `,`+Subdomain+`.`, -1)
			}
			if strings.Contains(newSubdomain, `//`+oldSubdomain+`.`) {
				newSubdomain = strings.Replace(newSubdomain, `//`+oldSubdomain+`.`, `//`+Subdomain+`.`, 1)
			}

			regex := regexp.MustCompile(`^` + oldSubdomain + `.\b`)
			newSubdomain = regex.ReplaceAllLiteralString(newSubdomain, Subdomain+`.`)

			d.Spec.Template.Spec.Containers[0].Env[idx].Value = newSubdomain
		}
		_, err = internal.Cfg.K8ss_local.ClientSet.AppsV1().Deployments(nsName).Update(context.Background(), d, metav1.UpdateOptions{})
		if err != nil {
			return err
		}
	}

	// //rolling restart affect deployments -- reticulum, hubs, and spoke
	// for _, dName := range []string{"reticulum", "hubs", "spoke"} {
	// 	d, err := internal.Cfg.K8ss_local.ClientSet.AppsV1().Deployments(ns).Get(context.Background(), dName, metav1.GetOptions{})
	// 	if err != nil {
	// 		return err
	// 	}
	// 	// touch annotation to trigger a restart ... https://github.com/kubernetes/kubectl/blob/release-1.16/pkg/polymorphichelpers/objectrestarter.go#L32
	// 	d.Spec.Template.ObjectMeta.Annotations = map[string]string{"turkeyorch-reboot-id": base64.RawURLEncoding.EncodeToString(internal.NewUUID())}

	// 	_, err = internal.Cfg.K8ss_local.ClientSet.AppsV1().Deployments(ns).Update(context.Background(), d, metav1.UpdateOptions{})
	// 	if err != nil {
	// 		return err
	// 	}
	// }

	// ^^^ rolling restart seems to sometimes cause haproxy to take a long time to refresh backend pods
	pods, err := internal.Cfg.K8ss_local.ClientSet.CoreV1().Pods(nsName).List(context.Background(), metav1.ListOptions{})
	if err != nil {
		return err
	}
	for _, pod := range pods.Items {
		err := internal.Cfg.K8ss_local.ClientSet.CoreV1().Pods(nsName).Delete(context.Background(), pod.Name, metav1.DeleteOptions{})
		if err != nil {
			return err
		}
	}
	// update ns label
	ns, err := internal.Cfg.K8ss_local.ClientSet.CoreV1().Namespaces().Get(context.Background(), nsName, metav1.GetOptions{})
	if err != nil {
		return err
	}
	ns.Labels["subdomain"] = Subdomain
	_, err = internal.Cfg.K8ss_local.ClientSet.CoreV1().Namespaces().Update(context.Background(), ns, metav1.UpdateOptions{})
	if err != nil {
		return err
	}

	// update ingress
	time.Sleep(15 * time.Second)
	internal.Logger.Sugar().Debugf("[hc_patch_subdomain.update ingress] %v => %v", oldSubdomain, Subdomain)
	ingress, err := internal.Cfg.K8ss_local.ClientSet.NetworkingV1().Ingresses(nsName).Get(context.Background(), "turkey-https", metav1.GetOptions{})
	if err != nil {
		return err
	}
	for i, rule := range ingress.Spec.Rules {
		hostArr := strings.SplitN(rule.Host, ".", 2)
		newHost := strings.Replace(hostArr[0], oldSubdomain, Subdomain, 1) + "." + hostArr[1]
		internal.Logger.Sugar().Debugf("hostArr[0]: %v, oldSubdomain: %v, Subdomain: %v", hostArr[0], oldSubdomain, Subdomain)
		if hostArr[0] != oldSubdomain {
			internal.Logger.Sugar().Warnf(" hostArr[0] != oldSubdomain,  hostArr[0]: %v, oldSubdomain: %v, Subdomain: %v", hostArr[0], oldSubdomain, Subdomain)
		}
		// newHost := Subdomain + "." + hostArr[1]
		ingress.Spec.Rules[i].Host = newHost
		internal.Logger.Sugar().Debugf("newHost: %v", newHost)
	}
	ingress.Annotations["haproxy.org/response-set-header"] = strings.Replace(
		ingress.Annotations["haproxy.org/response-set-header"],
		`//`+oldSubdomain+`.`, `//`+Subdomain+`.`, 1)
	_, err = internal.Cfg.K8ss_local.ClientSet.NetworkingV1().Ingresses(nsName).Update(context.Background(), ingress, metav1.UpdateOptions{})
	if err != nil {
		return err
	}

	return nil
}
