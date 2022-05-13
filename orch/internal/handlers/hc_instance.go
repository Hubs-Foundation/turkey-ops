package handlers

import (
	"context"
	"crypto/x509"
	"encoding/base64"
	"encoding/json"
	"encoding/pem"
	"errors"
	"fmt"
	"io/ioutil"
	"os"

	"net/http"
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

	//optional inputs
	Options string `json:"options"` //additional options, underscore(_)prefixed -- ie. "_ebs"

	//inherited from turkey cluster -- aka the values are here already, in internal.Cfg
	Domain               string `json:"domain"`
	DBname               string `json:"dbname"`
	DBpass               string `json:"dbpass"`
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
		status := r.URL.Query().Get("status")
		if status == "down" || status == "up" {
			hc_switch(w, r)
			return
		}
		http.Error(w, http.StatusText(http.StatusBadRequest), http.StatusBadRequest)
		return
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
	fileOption := "_gcs_sc"
	if os.Getenv("CLOUD") == "aws" {
		fileOption = "_s3fs"
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
	result := "created."
	// is subdomain available?
	nsList, _ := internal.Cfg.K8ss_local.ClientSet.CoreV1().Namespaces().List(context.Background(),
		metav1.ListOptions{LabelSelector: "Subdomain=" + hcCfg.Subdomain})
	if len(nsList.Items) != 0 {
		// if strings.HasSuffix(r.Header.Get("X-Forwarded-UserEmail"), "@mozilla.com") {
		// 	result += "[warning] subdomain already used by hub_id " + nsList.Items[0].Name + ", but was overridden for @mozilla.com user"
		// } else {
		internal.Logger.Error("error @ k8s pre-deployment checks: subdomain already exists: " + hcCfg.Subdomain)
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(map[string]interface{}{
			"result": "subdomain already exists",
			"hub_id": hcCfg.HubId,
		})
		return
		// }
	}

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

	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(map[string]interface{}{
		"result":        result,
		"useremail":     hcCfg.UserEmail,
		"hub_id":        hcCfg.HubId,
		"subdomain":     hcCfg.Subdomain,
		"tier":          hcCfg.Tier,
		"ccu_limit":     hcCfg.CcuLimit,
		"storage_limit": hcCfg.StorageLimit,
	})
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
			LabelSelector: "subdomain=" + cfg.Subdomain,
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
	//get r.body
	rBodyBytes, err := ioutil.ReadAll(r.Body)
	if err != nil {
		internal.Logger.Error("ERROR @ reading r.body, error = " + err.Error())
		return cfg, err
	}
	//make cfg
	err = json.Unmarshal(rBodyBytes, &cfg)
	if err != nil {
		internal.Logger.Error("bad hcCfg: " + string(rBodyBytes))
		return cfg, err
	}
	return cfg, nil
}

func makeHcCfg(r *http.Request) (hcCfg, error) {
	var cfg hcCfg

	//get r.body
	rBodyBytes, err := ioutil.ReadAll(r.Body)
	if err != nil {
		internal.Logger.Error("ERROR @ reading r.body, error = " + err.Error())
		return cfg, err
	}
	//make cfg
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
	cfg.DBpass = internal.Cfg.DBpass
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

	if hcCfg.Subdomain == "" {
		ns, err := internal.Cfg.K8ss_local.ClientSet.CoreV1().Namespaces().Get(context.Background(), "hc-"+hcCfg.HubId, metav1.GetOptions{})
		if err != nil {
			internal.Logger.Error("failed to get namespace for hubid " + hcCfg.HubId + "because: " + err.Error())
			return
		}
		hcCfg.Subdomain = ns.Labels["Subdomain"]
	}

	go func() {
		hcCfg.DBname = "ret_" + hcCfg.HubId
		internal.Logger.Debug("&#128024 deleting db: " + hcCfg.DBname)
		//delete db -- force
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
	}()

	go func() {
		nsName := "hc-" + hcCfg.HubId
		internal.Logger.Debug("&#128024 deleting ns: " + nsName)
		// //scale down deployments
		// ds, err := internal.Cfg.K8ss_local.ClientSet.AppsV1().Deployments(nsName).List(context.Background(), metav1.ListOptions{})
		// if err != nil {
		// 	internal.Logger.Error("failed to get deployments for namespace" + nsName + " because:" + err.Error())
		// } else {
		// 	for _, d := range ds.Items {
		// 		d.Spec.Replicas = pointerOfInt32(0)
		// 		_, err := internal.Cfg.K8ss_local.ClientSet.AppsV1().Deployments(nsName).Update(context.Background(), &d, metav1.UpdateOptions{})
		// 		if err != nil {
		// 			internal.Logger.Error("failed to scale down <ns: " + nsName + ", deployment: " + d.Name + ">: " + err.Error())
		// 		}
		// 	}
		// }
		//delete ns
		err = internal.Cfg.K8ss_local.ClientSet.CoreV1().Namespaces().Delete(context.TODO(),
			nsName,
			metav1.DeleteOptions{})
		if err != nil {
			internal.Logger.Error("delete ns failed: " + err.Error())
			return
		}
		internal.Logger.Debug("&#127754 deleted ns: " + nsName)
	}()

	go func() {
		err := internal.Cfg.Gcps.DeleteObjects("turkeyfs", hcCfg.Subdomain+"."+internal.Cfg.Domain)
		if err != nil {
			internal.Logger.Error("delete ns failed: " + err.Error())
			return
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

func hc_switch(w http.ResponseWriter, r *http.Request) {

	// sess := internal.GetSession(r.Cookie)

	cfg, err := getHcCfg(r)
	if err != nil {
		internal.Logger.Error("bad hcCfg: " + err.Error())
		w.WriteHeader(http.StatusNotFound)
		return
	}
	ns := "hc-" + cfg.HubId

	//acquire lock
	locker, err := internal.NewK8Locker(internal.NewK8sSvs_local().Cfg, ns)
	if err != nil {
		internal.Logger.Error("faild to acquire locker, try again later ... err: " + err.Error())
		return
	}

	locker.Lock()

	Replicas := 0
	status := r.URL.Query().Get("status")
	if status == "up" {
		// todo -- read tier, find out desired replica counts for each deployments (hubs, ret, spoke, ita, nearspark ... probably just ret)
		//			or, (possible?) just set them to 1 and let hpa taken care of the rest
		Replicas = 1
	}

	ds, err := internal.Cfg.K8ss_local.ClientSet.AppsV1().Deployments(ns).List(context.Background(), metav1.ListOptions{})
	if err != nil {
		internal.Logger.Error("wakeupHcNs - failed to list deployments: " + err.Error())
		return
	}

	for _, d := range ds.Items {
		d.Spec.Replicas = pointerOfInt32(Replicas)
		_, err := internal.Cfg.K8ss_local.ClientSet.AppsV1().Deployments(ns).Update(context.Background(), &d, metav1.UpdateOptions{})
		if err != nil {
			internal.Logger.Error("wakeupHcNs -- failed to scale <ns: " + ns + ", deployment: " + d.Name + "> back up: " + err.Error())
			return
		}
	}
	locker.Unlock()

}