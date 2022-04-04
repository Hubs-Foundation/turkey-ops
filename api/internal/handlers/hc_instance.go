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
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
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
	Domain         string `json:"domain"`
	DBname         string `json:"dbname"`
	DBpass         string `json:"dbpass"`
	PermsKey       string `json:"permskey"`
	SmtpServer     string `json:"smtpserver"`
	SmtpPort       string `json:"smtpport"`
	SmtpUser       string `json:"smtpuser"`
	SmtpPass       string `json:"smtppass"`
	GCP_SA_KEY_b64 string
	ItaChan        string

	//generated per instance
	JWK         string `json:"jwk"` // encoded from PermsKey.public
	GuardianKey string `json:"guardiankey"`
	PhxKey      string `json:"phxkey"`
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
		internal.GetLogger().Error("bad hcCfg: " + err.Error())
		http.Error(w, http.StatusText(http.StatusBadRequest), http.StatusBadRequest)
		return
	}

	// #2 render turkey-k8s-chart by apply cfg to hc.yam
	fileOption := "_gcs_sc"
	if os.Getenv("CLOUD") == "aws" {
		fileOption = "_s3fs"
	}
	internal.GetLogger().Debug(" >>>>>> selected option: " + fileOption)

	yamBytes, err := ioutil.ReadFile("./_files/yams/ns_hc" + fileOption + ".yam")
	if err != nil {
		internal.GetLogger().Error("failed to get ns_hc yam file because: " + err.Error())
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(map[string]interface{}{
			"result": "error @ getting k8s chart file",
			"hub_id": hcCfg.HubId,
		})
		return
	}

	renderedYamls, _ := internal.K8s_render_yams([]string{string(yamBytes)}, hcCfg)
	k8sChartYaml := renderedYamls[0]

	// #3 pre-deployment checks
	// is subdomain available?
	nsList, _ := internal.Cfg.K8ss_local.ClientSet.CoreV1().Namespaces().List(context.Background(),
		metav1.ListOptions{LabelSelector: "Subdomain=" + hcCfg.Subdomain})
	if len(nsList.Items) != 0 {
		internal.GetLogger().Error("error @ k8s pre-deployment checks: subdomain already exists: " + hcCfg.Subdomain)
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(map[string]interface{}{
			"result": "subdomain already exists",
			"hub_id": hcCfg.HubId,
		})
		return
	}

	// #4 kubectl apply -f <file.yaml> --server-side --field-manager "turkey-userid-<cfg.UserId>"
	internal.GetLogger().Debug("&#128640; --- deployment started for: " + hcCfg.HubId)
	err = internal.Ssa_k8sChartYaml(hcCfg.AccountId, k8sChartYaml, internal.Cfg.K8ss_local.Cfg)
	if err != nil {
		internal.GetLogger().Error("error @ k8s deploy: " + err.Error())
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(map[string]interface{}{
			"result": "error @ k8s deploy: " + err.Error(),
			"hub_id": hcCfg.HubId,
		})
		return
	}

	// // quality of life improvement for /console people
	// skipadminLink := "https://" + hcCfg.Subdomain + "." + hcCfg.Domain + "?skipadmin"
	// sess.Log("&#128640; --- deployment completed for: <a href=\"" +
	// 	skipadminLink + "\" target=\"_blank\"><b>&#128279;" + hcCfg.AccountId + ":" + hcCfg.Subdomain + "</b></a>")
	// sess.Log("&#128231; --- admin email: " + hcCfg.UserEmail)

	// #5 create db
	_, err = internal.PgxPool.Exec(context.Background(), "create database \""+hcCfg.DBname+"\"")
	if err != nil {
		if strings.Contains(err.Error(), "already exists (SQLSTATE 42P04)") {
			internal.GetLogger().Debug("db already exists: " + hcCfg.DBname)
		} else {
			internal.GetLogger().Error("error @ create hub db: " + err.Error())
			json.NewEncoder(w).Encode(map[string]interface{}{
				"result": "error @ create hub db: " + err.Error(),
				"hub_id": hcCfg.HubId,
			})
			return
		}
	}
	internal.GetLogger().Debug("&#128024; --- db created: " + hcCfg.DBname)

	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(map[string]interface{}{
		"result": "done",
		"hub_id": hcCfg.HubId,
	})

}

func hc_get(w http.ResponseWriter, r *http.Request) {
	sess := internal.GetSession(r.Cookie)

	cfg, err := getHcCfg(r)
	if err != nil {
		sess.Log("bad hcCfg: " + err.Error())
		return
	}

	//getting k8s config
	sess.Log("&#9989; ... using InClusterConfig")
	k8sCfg, err := rest.InClusterConfig()
	// }
	if k8sCfg == nil {
		sess.Log("ERROR" + err.Error())
		sess.Error(err.Error())
		return
	}
	sess.Log("&#129311; k8s.k8sCfg.Host == " + k8sCfg.Host)
	clientset, err := kubernetes.NewForConfig(k8sCfg)
	if err != nil {
		sess.Error(err.Error())
		return
	}
	//list ns
	nsList, err := clientset.CoreV1().Namespaces().List(context.TODO(),
		metav1.ListOptions{
			LabelSelector: "AccountId=" + cfg.AccountId,
		})
	if err != nil {
		sess.Error(err.Error())
		return
	}
	sess.Log("GET --- user <" + cfg.AccountId + "> owns: ")
	for _, ns := range nsList.Items {
		sess.Log("......ObjectMeta.GetName: " + ns.ObjectMeta.GetName())
		sess.Log("......ObjectMeta.Labels.dump: " + fmt.Sprint(ns.ObjectMeta.Labels))
	}
}

func getHcCfg(r *http.Request) (hcCfg, error) {
	var cfg hcCfg
	//get r.body
	rBodyBytes, err := ioutil.ReadAll(r.Body)
	if err != nil {
		fmt.Println("ERROR @ reading r.body, error = " + err.Error())
		return cfg, err
	}
	//make cfg
	err = json.Unmarshal(rBodyBytes, &cfg)
	if err != nil {
		fmt.Println("bad hcCfg: " + string(rBodyBytes))
		return cfg, err
	}
	return cfg, nil
}

func makeHcCfg(r *http.Request) (hcCfg, error) {
	var cfg hcCfg

	//get r.body
	rBodyBytes, err := ioutil.ReadAll(r.Body)
	if err != nil {
		fmt.Println("ERROR @ reading r.body, error = " + err.Error())
		return cfg, err
	}
	//make cfg
	err = json.Unmarshal(rBodyBytes, &cfg)
	if err != nil {
		fmt.Println("bad hcCfg: " + string(rBodyBytes))
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
		fmt.Println()
	}
	// HubId
	if cfg.HubId == "" {
		cfg.HubId = internal.PwdGen(6, int64(hash(cfg.AccountId+cfg.Subdomain)), "")
		//cfg.AccountId + "-" + strconv.FormatInt(time.Now().Unix()-1648957620, 36)
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
		internal.GetLogger().Error("failed to decode perms key")
		internal.GetLogger().Error("perms_key_str: " + perms_key_str)
		internal.GetLogger().Error(" cfg.PermsKey: " + cfg.PermsKey)
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
	cfg.ItaChan = internal.Cfg.Env

	//produc the rest
	cfg.GuardianKey = "strongSecret#1"
	cfg.PhxKey = "strongSecret#2"

	return cfg, nil
}

func hc_delete(w http.ResponseWriter, r *http.Request) {
	sess := internal.GetSession(r.Cookie)
	hcCfg, err := getHcCfg(r)
	if err != nil {
		sess.Error("bad hcCfg: " + err.Error())
		return
	}

	go func() {
		sess.Log("&#128024 deleting db: " + hcCfg.DBname)

		//delete db -- force
		force := true
		_, err = internal.PgxPool.Exec(context.Background(), "drop database "+hcCfg.DBname)
		if err != nil {
			if strings.Contains(err.Error(), "is being accessed by other users (SQLSTATE 55006)") && force {
				err = pg_kick_all(hcCfg, sess)
				if err != nil {
					sess.Error(err.Error())
					return
				}
				_, err = internal.PgxPool.Exec(context.Background(), "drop database "+hcCfg.DBname)
			}
			if err != nil {
				sess.Error(err.Error())
				return
			}
		}
		sess.Log("&#128024 deleted db: " + hcCfg.DBname)
	}()

	go func() {
		nsName := "hc-" + hcCfg.Subdomain
		sess.Log("&#128024 deleting ns: " + nsName)

		//getting k8s config
		sess.Log("&#9989; ... using InClusterConfig")
		k8sCfg, err := rest.InClusterConfig()
		// }
		if k8sCfg == nil {
			sess.Error(err.Error())
			return
		}
		sess.Log("&#129311; k8s.k8sCfg.Host == " + k8sCfg.Host)
		clientset, err := kubernetes.NewForConfig(k8sCfg)
		if err != nil {
			sess.Error("failed to get kubecfg" + err.Error())
			return
		}

		//delete ns
		err = clientset.CoreV1().Namespaces().Delete(context.TODO(),
			nsName,
			metav1.DeleteOptions{})
		if err != nil {
			sess.Error("delete ns failed: " + err.Error())
			return
		}
		sess.Log("&#127754 deleted ns: " + nsName)
	}()

	//return
	w.WriteHeader(http.StatusAccepted)
	json.NewEncoder(w).Encode(map[string]interface{}{
		"task":   "deletion",
		"hub_id": hcCfg.HubId,
	})
}

func pg_kick_all(cfg hcCfg, sess *internal.CacheBoxSessData) error {
	sqatterCount := -1
	tries := 0
	for sqatterCount != 0 && tries < 3 {
		squatters, _ := internal.PgxPool.Exec(context.Background(), `select usename,client_addr,state,query from pg_stat_activity where datname = '`+cfg.DBname+`'`)
		sess.Log("WARNING: pg_kick_all: kicking <" + fmt.Sprint(squatters.RowsAffected()) + "> squatters from " + cfg.DBname)
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
	sess := internal.GetSession(r.Cookie)
	cfg, err := getHcCfg(r)
	if err != nil {
		sess.Error("bad hcCfg: " + err.Error())
		w.WriteHeader(http.StatusNotFound)
		return
	}
	ns := "hc-" + cfg.Subdomain

	//acquire lock
	locker, err := internal.NewK8Locker(internal.NewK8sSvs_local().Cfg, ns)
	if err != nil {
		sess.Error("faild to acquire locker, try again later ... err: " + err.Error())
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

	ds, err := internal.Cfg.K8ss_local.ClientSet.AppsV1().Deployments(ns).List(context.Background(), v1.ListOptions{})
	if err != nil {
		sess.Error("wakeupHcNs - failed to list deployments: " + err.Error())
		return
	}

	for _, d := range ds.Items {
		d.Spec.Replicas = pointerOfInt32(Replicas)
		_, err := internal.Cfg.K8ss_local.ClientSet.AppsV1().Deployments(ns).Update(context.Background(), &d, v1.UpdateOptions{})
		if err != nil {
			sess.Error("wakeupHcNs -- failed to scale <ns: " + ns + ", deployment: " + d.Name + "> back up: " + err.Error())
			return
		}
	}
	locker.Unlock()

}
