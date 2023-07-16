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
	"os/exec"
	"regexp"
	"strconv"
	"sync/atomic"

	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/jackc/pgtype"
	networkingv1 "k8s.io/api/networking/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"main/internal"
)

type HCcfg struct {

	//required input
	UserEmail string `json:"useremail"`

	//required, but with fallbacks
	AccountId    string `json:"account_id"` // turkey account id, fallback to random string produced by useremail seeded rnd func
	HubId        string `json:"hub_id"`     // id of the hub instance, also used to name the hub's namespace, fallback to  random string produced by AccountId seeded rnd func
	FxaSub       string `json:"fxa_sub"`
	Name         string `json:"name"`
	Tier         string `json:"tier"`          // fallback to free
	CcuLimit     string `json:"ccu_limit"`     // fallback to 20
	StorageLimit string `json:"storage_limit"` // fallback to 0.5
	Subdomain    string `json:"subdomain"`     // fallback to HubId
	NodePool     string `json:"nodepool"`      // default == spot

	//optional inputs
	Options string `json:"options"` //additional options, debug purpose only, underscore(_)prefixed -- ie. "_nfs"
	Region  string `json:"region"`

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
	TENOR_API_KEY        string
	SENTRY_DSN_RET       string
	SENTRY_DSN_HUBS      string
	SENTRY_DSN_SPOKE     string
	//generated on the fly
	JWK          string `json:"jwk"`         // encoded from PermsKey.public
	GuardianKey  string `json:"guardiankey"` //ret's secret_key
	PhxKey       string `json:"phxkey"`      //ret's secret_key_base
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

	//control fields
	TurkeyJobReqMethod string `json:"turkey_job_req_method"`
	TurkeyJobJobId     string `json:"turkey_job_job_id"`
	TurkeyJobCallback  string `json:"turkey_job_callback"`
}

var HC_instance = http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {

	if r.URL.Path != "/hc_instance" {
		http.Error(w, http.StatusText(http.StatusNotFound), http.StatusNotFound)
		return
	}
	cfg, err := getHcCfg(r)
	if err != nil {
		http.Error(w, http.StatusText(http.StatusBadRequest)+":"+err.Error(), http.StatusBadRequest)
		return
	}

	if r.Method == "POST" && cfg.Tier == "p0" && cfg.Region == "" {
		http.Error(w, "no new p0 instance on root cluster pls, try again with a region field in the json payload", http.StatusBadRequest)
		return
	}

	internal.Logger.Sugar().Debugf("cfg: %v", cfg)
	if cfg.Region != "" || (cfg.Domain != "" && cfg.Domain != internal.Cfg.HubDomain) {
		// multi-cluster request
		cfg, err = handleMultiClusterReq(w, r, cfg)
		if err != nil {
			internal.Logger.Sugar().Errorf("failed @ handleMultiClusterReq, err:= ", err)
			w.WriteHeader(http.StatusInternalServerError)
		}
	} else {
		// local hc_instance request
		err = handle_hc_instance_req(r, cfg)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
		}
		json.NewEncoder(w).Encode(map[string]interface{}{
			"result": "done",
			"hub_id": cfg.HubId,
			"domain": cfg.HubDomain,
			"region": cfg.Region,
		})
	}

	if err != nil && cfg.AccountId != "" && internal.Cfg.IsRoot {
		//update orchDb
		task := hc_task_translator(r)
		switch task {
		case "hc_create":
			accountId, err := strconv.ParseInt(cfg.AccountId, 10, 64)
			if err != nil {
				internal.Logger.Sugar().Errorf("accountId cannot be parsed into int64: %v", cfg.AccountId)
				return
			}
			hubId, err := strconv.ParseInt(cfg.HubId, 10, 64)
			if err != nil {
				internal.Logger.Sugar().Warnf("failed to convert cfg.HubId(%v)", hubId)
				hubId = time.Now().UnixNano()
				internal.Logger.Sugar().Warnf("using time.Now().UnixNano() (%v)", hubId)
			}
			OrchDb_upsertHub(
				Turkeyorch_hubs{
					Hub_id:      pgtype.Int8{Int: int64(hubId)},
					Account_id:  pgtype.Int8{Int: accountId},
					Fxa_sub:     pgtype.Text{String: cfg.FxaSub},
					Name:        pgtype.Text{String: cfg.Name},
					Tier:        pgtype.Text{String: cfg.Tier},
					Status:      pgtype.Text{String: "ready"},
					Email:       pgtype.Text{String: cfg.UserEmail},
					Subdomain:   pgtype.Text{String: cfg.Subdomain},
					Inserted_at: pgtype.Timestamptz{Time: time.Now()},
					Domain:      pgtype.Text{String: cfg.Domain},
					Region:      pgtype.Text{String: cfg.Region},
				})
		case "hc_delete":
			OrchDb_deleteHub(cfg.HubId)
		case "hc_switch_up":
			OrchDb_updateHub_status(cfg.HubId, "up")
		case "hc_switch_down":
			OrchDb_updateHub_status(cfg.HubId, "down")
		case "hc_collect":
			OrchDb_updateHub_status(cfg.HubId, "collected")
		case "hc_restore":
			OrchDb_updateHub_status(cfg.HubId, "ready")
		case "hc_update":
			if cfg.Tier != "" && cfg.CcuLimit != "" && cfg.StorageLimit != "" {
				OrchDb_updateHub_tier(cfg.HubId, cfg.Tier)
			}
			if cfg.Subdomain != "" {
				OrchDb_updateHub_subdomain(cfg.HubId, cfg.Subdomain)
			}
		}
	}
})

func handle_hc_instance_req(r *http.Request, cfg HCcfg) error {
	var err error
	task := hc_task_translator(r)
	switch task {
	case "hc_create":
		hcCfg, err := makeHcCfg(cfg)
		if err != nil {
			return fmt.Errorf("bad cfg, err: %v", err)
		}
		err = CreateHubsCloudInstance(hcCfg)
		if err != nil {
			return fmt.Errorf("failed to create, err: %v", err)
		}
		return nil
	case "hc_delete":
		if cfg.HubId == "" {
			return fmt.Errorf("missing hcCfg.HubId, err")
		}
		DeleteHubsCloudInstance(cfg.HubId, false, false)
		return nil
	case "hc_switch_up":
		err := hc_switch(cfg.HubId, "up")
		if err != nil {
			return fmt.Errorf("failed @ hc_switch: %v", err)
		}
	case "hc_switch_down":
		err := hc_switch(cfg.HubId, "down")
		if err != nil {
			return fmt.Errorf("failed @ hc_switch: %v", err)
		}
	case "hc_collect":
		if cfg.HubId == "" || cfg.Subdomain == "" || cfg.Tier == "" || cfg.UserEmail == "" ||
			cfg.GuardianKey == "" || cfg.PhxKey == "" {
			return fmt.Errorf("bad cfg : %v", cfg)
		}
		err = hc_collect(cfg)
		if err != nil {
			return fmt.Errorf("failed @ hc_collect: %v", err)
		}
	case "hc_restore":
		err = hc_restore(cfg.Subdomain)
		if err != nil {
			return fmt.Errorf("failed @ hc_restore: %v", err)
		}
	case "hc_update":
		msg, err := UpdateHubsCloudInstance(cfg)
		if err != nil {
			return fmt.Errorf("failed @hc_update: %v", err)
		}
		internal.Logger.Debug("UpdateHubsCloudInstance: " + msg)
		return nil
	default:
		return errors.New("not implemented")
	}
	return err
}

func hc_collect(cfg HCcfg) error {
	atomic.AddInt32(&internal.RunningTask, 1)
	defer atomic.AddInt32(&internal.RunningTask, -1)

	nsName := "hc-" + cfg.HubId
	hubDir := "/turkeyfs/" + nsName

	err := ioutil.WriteFile(hubDir+"/collecting", []byte(time.Now().Format(time.RFC3339)), 0644)
	if err != nil {
		return err
	}

	// backup -- config
	cfgJsonBytes, err := json.Marshal(cfg)
	if err != nil {
		return err
	}
	err = ioutil.WriteFile(hubDir+"/cfg.json", cfgJsonBytes, 0644)
	if err != nil {
		return err
	}

	// backup -- dump db to pgDumpFile
	dbName := "ret_" + cfg.HubId
	pgDumpFile := dbName + ".sql"
	cmd := exec.Command(
		"pg_dump", internal.Cfg.DBconn+"/"+dbName,
		"-f", hubDir+"/"+pgDumpFile,
	)
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	err = cmd.Run()
	internal.Logger.Sugar().Debugf("stdout: %v, stderr: %v", stdout.String(), stderr.String())
	if err != nil {
		return fmt.Errorf("failed to execute pg_dump: %v", err)
	}

	// add to subdomain:hubId lookup table
	internal.RetryFunc(15*time.Second, 3*time.Second,
		func() error {
			trcCm, err := internal.Cfg.K8ss_local.GetOrCreateTrcConfigmap()
			if err != nil {
				return err
			}
			trcCm.Data[cfg.Subdomain] = cfg.HubId
			_, err = internal.Cfg.K8ss_local.ClientSet.CoreV1().ConfigMaps(internal.Cfg.PodNS).Update(context.Background(), trcCm, metav1.UpdateOptions{})
			return err
		})
	if err != nil {
		return err
	}

	//delete, keepData == true
	deleting, err := DeleteHubsCloudInstance(cfg.HubId, true, false)
	if err != nil {
		return err
	}
	for m := range deleting { //wait for completion
		internal.Logger.Debug(m)
	}
	err = os.Remove(hubDir + "/collecting")
	if err != nil {
		return err
	}

	// all done -- add route
	internal.RetryFunc(15*time.Second, 3*time.Second,
		func() error {
			trcIg, err := internal.Cfg.K8ss_local.GetOrCreateTrcIngress()
			if err != nil {
				return err
			}
			pathType := networkingv1.PathTypePrefix
			trcIg.Spec.Rules = append(
				trcIg.Spec.Rules,
				networkingv1.IngressRule{
					Host: cfg.Subdomain + "." + internal.Cfg.HubDomain,
					IngressRuleValue: networkingv1.IngressRuleValue{
						HTTP: &networkingv1.HTTPIngressRuleValue{
							Paths: []networkingv1.HTTPIngressPath{
								{
									Path:     "/",
									PathType: &pathType,
									Backend: networkingv1.IngressBackend{
										Service: &networkingv1.IngressServiceBackend{
											Name: "turkeyorch",
											Port: networkingv1.ServiceBackendPort{
												Number: 888,
											}}}},
							}}}})
			_, err = internal.Cfg.K8ss_local.ClientSet.NetworkingV1().Ingresses(internal.Cfg.PodNS).Update(context.Background(), trcIg, metav1.UpdateOptions{})
			return err
		})
	if err != nil {
		return err
	}

	// todo -- multi-cluster status mgmt
	// OrchDb_updateHub_status(cfg.HubId, "collected")

	return nil
}

var hc_restore_cooldown = 30 * time.Minute

func hc_restore(hubId string) error {
	atomic.AddInt32(&internal.RunningTask, 1)
	defer atomic.AddInt32(&internal.RunningTask, -1)
	// //find hubId
	// trcCm, err := internal.Cfg.K8ss_local.GetOrCreateTrcConfigmap()
	// if err != nil {
	// 	return err
	// }
	// hubId := trcCm.Data[subdomain]
	// if hubId == "" {
	// 	return errors.New("failed to get hubId for subdomain: %v" + subdomain)
	// }
	nsName := "hc-" + hubId
	hubDir := "/turkeyfs/" + nsName
	internal.Logger.Sugar().Debugf("nsName: %v, hubDir: %v", nsName, hubDir)

	// waitTtl := 15 * time.Minute
	// for {
	// 	_, err := os.Stat(hubDir + "/collecting")
	// 	if os.IsNotExist(err) {
	// 		break
	// 	}
	// 	time.Sleep(1 * time.Minute)
	// 	waitTtl -= 1 * time.Minute
	// }

	if _, err := os.Stat(hubDir + "/collecting"); err == nil {
		return errors.New("***blocked ... try again later")
	}

	//check cooldown
	if tsBytes, err := ioutil.ReadFile(hubDir + "/trc_ts"); err == nil {
		t, err := time.Parse(time.RFC3339, string(tsBytes))
		if err != nil {
			internal.Logger.Sugar().Errorf("failed to deserialize time: %s", err)
			return fmt.Errorf("failed to deserialize time: %s", err)
		}

		if time.Since(t) < 10*time.Minute {
			return fmt.Errorf("***_ok_")
		}
		cooldownLeft := hc_restore_cooldown - time.Since(t)
		if cooldownLeft > 0 {
			return fmt.Errorf("***cooldown in progress -- try again in %vs", strings.Split(cooldownLeft.String(), ".")[0])
		}
	}
	// get configs
	cfgBytes, err := ioutil.ReadFile(hubDir + "/cfg.json")
	if err != nil {
		if _, err := os.Stat(hubDir + "/cfg.json.wip"); err == nil {
			internal.Logger.Warn("hc_restore already in progress (started by another orch instance?)")
			return fmt.Errorf("***working on it")
		}
		return err
	}

	// create db
	dBname := "ret_" + hubId
	_, err = internal.PgxPool.Exec(context.Background(), "create database \""+dBname+"\"")
	if err != nil {
		if strings.Contains(err.Error(), "already exists (SQLSTATE 42P04)") {
			internal.Logger.Sugar().Warn("db already exists: %v", err)
		} else {
			internal.Logger.Sugar().Errorf("unexpected error @ create db: %v", err)
			return err
		}
	}

	cfg := HCcfg{}
	err = json.Unmarshal(cfgBytes, &cfg)
	if err != nil {
		return err
	}
	os.Rename(hubDir+"/cfg.json", hubDir+"/cfg.json.wip")

	// restore db
	dbName := "ret_" + cfg.HubId
	pgDumpFile := hubDir + "/" + dbName + ".sql"
	dbCmd := exec.Command("psql", internal.Cfg.DBconn+"/"+dbName, "-f", pgDumpFile)
	out, err := dbCmd.CombinedOutput()
	if err != nil {
		internal.Logger.Sugar().Errorf("failed: %v, %v", err, out)
		return fmt.Errorf("failed to restore db. <err>: %v, <output>: %v", err, string(out))
	}
	// internal.Logger.Debug("dbCmd.out: " + string(out))

	// recreate hub
	cfg, err = makeHcCfg(cfg)
	if err != nil {
		return fmt.Errorf("bad cfg, err: %v", err)
	}
	err = CreateHubsCloudInstance(cfg)
	if err != nil {
		internal.Logger.Sugar().Errorf("Failed to recreate: %s", err)
		return fmt.Errorf("failed to create, err: %v", err)
	}

	err = ioutil.WriteFile(hubDir+"/trc_ts", []byte(time.Now().Format(time.RFC3339)), 0644)
	if err != nil {
		internal.Logger.Sugar().Errorf("Failed writing trc_ts file: %s", err)
		return fmt.Errorf("failed writing trc_ts file: %s", err)
	}

	go func() { // drop route after 10 sec
		time.Sleep(10 * time.Second)
		internal.RetryFunc(15*time.Second, 3*time.Second, func() error {
			trcIg, err := internal.Cfg.K8ss_local.GetOrCreateTrcIngress()
			if err != nil {
				return err
			}
			for idx, igRule := range trcIg.Spec.Rules {
				if igRule.Host == cfg.Subdomain+"."+internal.Cfg.HubDomain {
					trcIg.Spec.Rules = append(trcIg.Spec.Rules[:idx], trcIg.Spec.Rules[idx+1:]...)
					break
				}
			}
			_, err = internal.Cfg.K8ss_local.ClientSet.NetworkingV1().Ingresses(internal.Cfg.PodNS).Update(context.Background(),
				trcIg, metav1.UpdateOptions{})
			return err
		})
	}()

	err = os.Remove(hubDir + "/cfg.json.wip")

	return err
}

func UpdateHubsCloudInstance(cfg HCcfg) (string, error) {
	atomic.AddInt32(&internal.RunningTask, 1)
	defer atomic.AddInt32(&internal.RunningTask, -1)

	// tier change
	if cfg.Tier != "" && cfg.CcuLimit != "" && cfg.StorageLimit != "" {
		go func() {
			if cfg.Tier == "p0" && cfg.Subdomain != "" {
				internal.Logger.Sugar().Debugf("hc_updateTier[p0] / hc_patch_subdomain, hub_id: %v, subdomain: %v", cfg.HubId, cfg.Subdomain)
				err := hc_patch_subdomain(cfg.HubId, cfg.Subdomain)
				if err != nil {
					internal.Logger.Error("hc_updateTier[p0] / hc_patch_subdomain FAILED: " + err.Error())
				}
				time.Sleep(3 * time.Second)
			}
			err := hc_updateTier(cfg)
			if err != nil {
				internal.Logger.Error("hc_updateTier FAILED: " + err.Error())
			}
		}()
		return "tier update started for: " + cfg.HubId, nil
	}

	// subdomain updates
	if cfg.Subdomain != "" {
		go func() {
			err := hc_patch_subdomain(cfg.HubId, cfg.Subdomain)
			if err != nil {
				internal.Logger.Error("hc_patch_subdomain FAILED: " + err.Error())
			}
		}()
		return "subdomain update started for: " + cfg.HubId, nil
	}

	return "bad request", fmt.Errorf("bad req -- cfg: %v", cfg)
}

func CreateHubsCloudInstance(hcCfg HCcfg) error {
	atomic.AddInt32(&internal.RunningTask, 1)
	defer atomic.AddInt32(&internal.RunningTask, -1)
	// #1.1 pre-deployment checks
	nsList, _ := internal.Cfg.K8ss_local.ClientSet.CoreV1().Namespaces().List(context.Background(),
		metav1.ListOptions{LabelSelector: "hub_id=" + hcCfg.HubId})
	if len(nsList.Items) != 0 {
		internal.Logger.Error("hub_id already exists: " + hcCfg.HubId)
		return fmt.Errorf("bounce -- hub_id already exists")
	}

	// #2 render turkey-k8s-chart by apply cfg to hc.yam
	fileOption := "_gfs" //default
	// if hcCfg.Tier == "free" { //default for free tier
	// 	fileOption = "_nfs"
	// }
	//override
	if hcCfg.Options != "" {
		fileOption = hcCfg.Options
	}

	isNew := false
	if fileOption == "_fs" || fileOption == "_gfs" { //create folder for hub-id (hc-<hub_id>) in turkeyfs
		hubDir := "/turkeyfs/hc-" + hcCfg.HubId
		if _, err := os.Stat(hubDir); err != nil { // create if not exist
			os.MkdirAll(hubDir, 0600)
			isNew = true
		}
	}

	internal.Logger.Debug(" >>>>>> selected option: " + fileOption)

	yamBytes, err := ioutil.ReadFile("./_files/yams/ns_hc" + fileOption + ".yam")
	if err != nil {
		internal.Logger.Error("failed to get ns_hc yam file because: " + err.Error())
		return errors.New("error @ getting k8s chart file: " + err.Error())
	}

	renderedYamls, err := internal.K8s_render_yams([]string{string(yamBytes)}, hcCfg)
	if err != nil {
		internal.Logger.Error("failed to render ns_hc yam file because: " + err.Error())
		return errors.New("error @ rendering k8s chart file: " + err.Error())
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
	err = internal.Ssa_k8sChartYaml(fmt.Sprintf("%v", hcCfg.AccountId), k8sChartYaml, internal.Cfg.K8ss_local.Cfg)
	if err != nil {
		internal.Logger.Error("error @ k8s deploy: " + err.Error())
		return errors.New("error @ k8s deploy: " + err.Error())
	}

	// // quality of life improvement for /console people
	// skipadminLink := "https://" + hcCfg.Subdomain + "." + hcCfg.Domain + "?skipadmin"
	// internal.Logger.Debug("&#128640; --- deployment completed for: <a href=\"" +
	// 	skipadminLink + "\" target=\"_blank\"><b>&#128279;" + hcCfg.AccountId + ":" + hcCfg.Subdomain + "</b></a>")
	// internal.Logger.Debug("&#128231; --- admin email: " + hcCfg.UserEmail)

	// #5 create db
	createDB_tries := 3
	for {
		_, err = internal.PgxPool.Exec(context.Background(), "create database \""+hcCfg.DBname+"\"")
		if err == nil {
			break
		}
		if strings.Contains(err.Error(), "already exists (SQLSTATE 42P04)") {
			internal.Logger.Debug("db already exists: " + hcCfg.DBname)
			if isNew {
				internal.Logger.Error("db already exists but isNew, bad!!!, manual investigation needed")
				isNew = false
			}
			break
		}
		if createDB_tries > 0 {
			internal.Logger.Sugar().Warnf("failed @ create db (%v), tries left: %v", err, createDB_tries)
		} else {
			return errors.New("error @ create db: " + err.Error())
		}
	}
	internal.Logger.Debug("&#128024; --- db : " + hcCfg.DBname)

	if isNew {
		go func() {
			// temporary api-automation hacks for until this is properly implemented in reticulum
			err = post_creation_hacks(hcCfg)
			if err != nil {
				internal.Logger.Error("post_creation_hacks FAILED: " + err.Error())
			}
			// #6 enforce tiers
			hc_updateTier(hcCfg)
		}()
	}
	return nil
}

func post_creation_hacks(cfg HCcfg) error {

	_httpClient := &http.Client{
		Timeout:   5 * time.Second,
		Transport: &http.Transport{TLSClientConfig: &tls.Config{InsecureSkipVerify: true}},
	}
	//wait for ret
	retReq, _ := http.NewRequest("GET", "https://"+cfg.Subdomain+"."+cfg.HubDomain+"/health", nil)

	_, took, err := internal.RetryHttpReq(_httpClient, retReq, 6*time.Minute)
	if err != nil {
		return err
	}

	internal.Logger.Sugar().Debugf("tReady: %v, hubId: %v", took, cfg.HubId)

	//get admin auth token
	token, err := ret_getAdminToken(cfg)
	if err != nil {
		return err
	}

	err = ret_setDefaultTheme(token, cfg)
	if err != nil {
		internal.Logger.Error("ret_setDefaultTheme failed: " + err.Error())
		//return
	}

	//load assets
	assetPackUrl := strings.Replace(internal.Cfg.HC_INIT_ASSET_PACK, "turkey-init.pack", "turkey-init-"+cfg.Tier+".pack", 1)
	internal.Logger.Debug("loading assetPackUrl: " + assetPackUrl)
	resp, err := http.Get(assetPackUrl)
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
		//
		err = ret_load_asset(url, cfg, string(token))
		if err != nil {
			internal.Logger.Error(fmt.Sprintf("failed to load asset: %v, error: %v", url, err))
		}

	}
	return nil
}

func ret_load_asset(url *url.URL, cfg HCcfg, token string) error {
	pathArr := strings.Split(url.Path, "/")
	if len(pathArr) < 3 {
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
		"http://ret.hc-"+cfg.HubId+":4001/api/v1/"+kind_s,
		bytes.NewBuffer([]byte(`{"url":"`+assetUrl+`"}`)),
	)
	loadReq.Header.Add("content-type", "application/json")
	loadReq.Header.Add("authorization", "bearer "+string(token))

	_httpClient := &http.Client{
		Timeout:   15 * time.Second,
		Transport: &http.Transport{TLSClientConfig: &tls.Config{InsecureSkipVerify: true}},
	}
	resp, took, err := internal.RetryHttpReq(_httpClient, loadReq, 30*time.Second)
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
		"http://ret.hc-"+cfg.HubId+":4001/api/postgrest/"+kind_s+"?"+kind+"_sid=ilike.*"+newAssetId+"*",
		// bytes.NewBuffer([]byte(`{"url":"`+assetUrl+`"}`)),
		nil,
	)
	getReq.Header.Add("authorization", "bearer "+string(token))
	// resp, err = _httpClient.Do(getReq)
	resp, took, err = internal.RetryHttpReq(_httpClient, getReq, 30*time.Second)
	if err != nil {
		return err
	}
	internal.Logger.Sugar().Debugf("### get -- took: %v, loaded: %v, new_id: %v", took, assetUrl, newAssetId)

	getReqBody, _ := ioutil.ReadAll(resp.Body)
	if kind == "avatar" {
		err := ret_avatar_post_import(getReqBody, cfg.Subdomain, cfg.HubDomain, token)
		if err != nil {
			return err
		}
	} else if kind == "scene" {
		err := ret_scene_post_import(getReqBody, cfg.Subdomain, cfg.HubDomain, token)
		if err != nil {
			return err
		}
	} else {
		return errors.New("unexpected kind: " + kind)
	}

	return nil
}

// func hc_get(w http.ResponseWriter, r *http.Request) {
// 	// sess := internal.GetSession(r.Cookie)

// 	cfg, err := getHcCfg(r)
// 	if err != nil {
// 		internal.Logger.Debug("bad hcCfg: " + err.Error())
// 		return
// 	}

// 	//getting k8s config
// 	internal.Logger.Debug("&#9989; ... using InClusterConfig")
// 	k8sCfg, err := rest.InClusterConfig()
// 	// }
// 	if k8sCfg == nil {
// 		internal.Logger.Debug("ERROR" + err.Error())
// 		internal.Logger.Error(err.Error())
// 		return
// 	}
// 	internal.Logger.Debug("&#129311; k8s.k8sCfg.Host == " + k8sCfg.Host)
// 	clientset, err := kubernetes.NewForConfig(k8sCfg)
// 	if err != nil {
// 		internal.Logger.Error(err.Error())
// 		return
// 	}
// 	//list ns
// 	nsList, err := clientset.CoreV1().Namespaces().List(context.TODO(),
// 		metav1.ListOptions{
// 			LabelSelector: "hub_id" + cfg.HubId,
// 		})
// 	if err != nil {
// 		internal.Logger.Error(err.Error())
// 		return
// 	}
// 	internal.Logger.Debug("GET --- user <" + cfg.AccountId + "> owns: ")
// 	for _, ns := range nsList.Items {
// 		internal.Logger.Debug("......ObjectMeta.GetName: " + ns.ObjectMeta.GetName())
// 		internal.Logger.Debug("......ObjectMeta.Labels.dump: " + fmt.Sprint(ns.ObjectMeta.Labels))
// 	}
// }

func getHcCfg(r *http.Request) (HCcfg, error) {
	var cfg HCcfg
	rBodyBytes, err := ioutil.ReadAll(r.Body)
	if err != nil {
		internal.Logger.Error("ERROR @ reading r.body, error = " + err.Error())
		return cfg, err
	}
	// internal.Logger.Sugar().Debugf("rBodyBytes: " + string(rBodyBytes))
	err = json.Unmarshal(rBodyBytes, &cfg)
	if err != nil {
		internal.Logger.Error("bad hcCfg: " + string(rBodyBytes))
		return cfg, err
	}
	return cfg, nil
}

func makeHcCfg(cfg HCcfg) (HCcfg, error) {
	// //userEmail supplied?
	// if cfg.UserEmail == "" {
	// 	cfg.UserEmail = r.Header.Get("X-Forwarded-UserEmail") //try fallback to authenticated useremail
	// }
	//verify format?
	if cfg.UserEmail == "" { // can't create without email
		return cfg, errors.New("bad input, missing UserEmail or X-Forwarded-UserEmail")
		// cfg.UserEmail = "fooooo@barrrr.com"
	}

	// AccountId
	if cfg.AccountId == "" {
		// cfg.AccountId = internal.PwdGen(8, int64(hash(cfg.UserEmail)), "")
		// cfg.AccountId = 0
		// internal.Logger.Sugar().Debugf("AccountId unspecified, using: %v", cfg.AccountId)
		internal.Logger.Warn("AccountId unspecified, will not write orchDb")

	}
	// HubId
	if cfg.HubId == "" {
		return cfg, errors.New("bad cfg -- missing HubId")
		// cfg.HubId = internal.PwdGen(6, int64(hash(cfg.AccountId+cfg.Subdomain)), "")
		// internal.Logger.Debug("HubId unspecified, using: " + cfg.HubId)
	}
	//default Tier is free
	if cfg.Tier == "" {
		cfg.Tier = "p0"
	}
	if cfg.Tier == "early_access" {
		cfg.Tier = "p1"
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
	cfg.Region = internal.Cfg.Region
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
	cfg.TENOR_API_KEY = internal.Cfg.TENOR_API_KEY
	cfg.SENTRY_DSN_RET = internal.Cfg.SENTRY_DSN_RET
	cfg.SENTRY_DSN_HUBS = internal.Cfg.SENTRY_DSN_HUBS
	cfg.SENTRY_DSN_SPOKE = internal.Cfg.SENTRY_DSN_SPOKE

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

func DeleteHubsCloudInstance(hubId string, keepFiles bool, keepDB bool) (chan (string), error) {
	atomic.AddInt32(&internal.RunningTask, 1)
	defer atomic.AddInt32(&internal.RunningTask, -1)

	//mark the hc- namespace for the cleanup cronjob (todo)
	nsName := "hc-" + hubId
	err := internal.Cfg.K8ss_local.PatchNsAnnotation(nsName, "deleting", "true")
	if err != nil {
		internal.Logger.Warn("failed @PatchNsAnnotation, err: " + err.Error())
		// return nil, errors.New("failed @PatchNsAnnotation, err: " + err.Error())
	}
	// //remove ingress route
	// err = internal.Cfg.K8ss_local.ClientSet.NetworkingV1().Ingresses(nsName).DeleteCollection(context.Background(), metav1.DeleteOptions{}, metav1.ListOptions{})
	// if err != nil {
	// 	internal.Logger.Error("failed @k8s.ingress.DeleteCollection, err: " + err.Error())
	// }
	// ttl := 1 * time.Minute
	// wait := 5 * time.Second
	// for il, _ := internal.Cfg.K8ss_local.ClientSet.NetworkingV1().Ingresses(nsName).List(context.Background(), metav1.ListOptions{}); len(il.Items) > 0; {
	// 	time.Sleep(5 * time.Second)
	// 	if ttl -= wait; ttl < 0 {
	// 		internal.Logger.Error("timeout @ remove ingress")
	// 		break
	// 	}
	// }

	deleting := make(chan string)
	go func() {
		internal.Logger.Debug("&#128024 deleting ns: " + nsName)
		// scale down the namespace before deletion to avoid pod/ns "stuck terminating"
		select {
		case deleting <- "scaling down " + nsName:
		default:
		}
		hc_switch(hubId, "down")
		err := internal.Cfg.K8ss_local.WaitForPodKill(nsName, 60*time.Minute, 1)
		if err != nil {
			internal.Logger.Error("error @WaitForPodKill: " + err.Error())
			// close(deleting)
			// return
		}
		//delete ns
		select {
		case deleting <- "deleting ns" + nsName:
		default:
		}
		err = internal.Cfg.K8ss_local.ClientSet.CoreV1().Namespaces().Delete(context.TODO(),
			nsName,
			metav1.DeleteOptions{})
		if err != nil {
			internal.Logger.Error("delete ns failed: " + err.Error())
			// close(deleting)
			// return
		}
		internal.Logger.Debug("&#127754 deleted ns: " + nsName)

		if !keepFiles {
			select {
			case deleting <- "deleting files":
			default:
			}
			if len(hubId) < 1 {
				internal.Logger.Error("DANGER!!! empty HubId")
			} else {
				os.RemoveAll("/turkeyfs/hc-" + hubId)
			}
			err = internal.Cfg.Gcps.GCS_DeleteObjects("turkeyfs", "hc-"+hubId+"."+internal.Cfg.Domain)
			if err != nil {
				internal.Logger.Error("delete ns failed: " + err.Error())
				// close(deleting)
				// return
			}
		}
		if !keepDB {
			select {
			case deleting <- "deleting db":
			default:
			}
			dbName := "ret_" + hubId
			internal.Logger.Debug("&#128024 deleting db: " + dbName)
			force := true
			_, err = internal.PgxPool.Exec(context.Background(), "drop database "+dbName)
			if err != nil {
				if strings.Contains(err.Error(), "is being accessed by other users (SQLSTATE 55006)") && force {
					err = pg_kick_all(dbName)
					if err != nil {
						internal.Logger.Error(err.Error())
						close(deleting)
						return
					}
					_, err = internal.PgxPool.Exec(context.Background(), "drop database "+dbName)
				}
				if err != nil {
					internal.Logger.Error(err.Error())
					// close(deleting)
					// return
				}
			}
			internal.Logger.Debug("&#128024 deleted db: " + dbName)
		}
		select {
		case deleting <- "done deleting: " + nsName:
		default:
		}
		close(deleting)
	}()
	return deleting, nil
}

func pg_kick_all(dbName string) error {

	sqatterCount := -1
	tries := 0
	for sqatterCount != 0 && tries < 3 {
		squatters, _ := internal.PgxPool.Exec(context.Background(), `select usename,client_addr,state,query from pg_stat_activity where datname = '`+dbName+`'`)
		internal.Logger.Debug("WARNING: pg_kick_all: kicking <" + fmt.Sprint(squatters.RowsAffected()) + "> squatters from " + dbName)
		_, _ = internal.PgxPool.Exec(context.Background(), `REVOKE CONNECT ON DATABASE `+dbName+` FROM public`)
		_, _ = internal.PgxPool.Exec(context.Background(), `REVOKE CONNECT ON DATABASE `+dbName+` FROM `+internal.Cfg.DBuser)
		_, _ = internal.PgxPool.Exec(context.Background(), `SELECT pg_terminate_backend (pg_stat_activity.pid) FROM pg_stat_activity WHERE pg_stat_activity.datname = '`+dbName+`'`)
		time.Sleep(3 * time.Second)
		squatters, _ = internal.PgxPool.Exec(context.Background(), `select usename,client_addr,state,query from pg_stat_activity where datname = '`+dbName+`'`)
		sqatterCount = int(squatters.RowsAffected())
		tries++
	}
	if sqatterCount != 0 {
		return errors.New("ERROR: pg_kick_all: failed to kick <" + fmt.Sprint(sqatterCount) + "> squatter(s): ")
	}
	return nil
}

func hc_switch(HubId, status string) error {
	atomic.AddInt32(&internal.RunningTask, 1)
	defer atomic.AddInt32(&internal.RunningTask, -1)

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

		d.ResourceVersion = ""
		_, err = internal.Cfg.K8ss_local.ClientSet.AppsV1().Deployments(nsName).Update(context.Background(), d, metav1.UpdateOptions{})
		if err != nil {
			return err
		}
	}
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
	// update ingress
	time.Sleep(5 * time.Second)
	ingresses, err := internal.Cfg.K8ss_local.ClientSet.NetworkingV1().Ingresses(nsName).List(context.Background(), metav1.ListOptions{})
	if err != nil {
		return err
	}
	for _, ingress := range ingresses.Items {
		internal.Logger.Sugar().Debugf("updating ingress: " + ingress.Name)
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

		ingress.ResourceVersion = ""
		_, err = internal.Cfg.K8ss_local.ClientSet.NetworkingV1().Ingresses(nsName).Update(context.Background(), &ingress, metav1.UpdateOptions{})
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

	ns.ResourceVersion = ""
	_, err = internal.Cfg.K8ss_local.ClientSet.CoreV1().Namespaces().Update(context.Background(), ns, metav1.UpdateOptions{})
	if err != nil {
		return err
	}

	return nil
}
