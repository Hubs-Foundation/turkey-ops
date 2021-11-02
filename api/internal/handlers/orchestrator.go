package handlers

import (
	"bytes"
	"context"
	"crypto/x509"
	"encoding/json"
	"encoding/pem"
	"errors"
	"fmt"
	"io"
	"io/ioutil"
	"os"

	"net/http"
	"strconv"
	"strings"
	"text/template"
	"time"

	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/serializer/yaml"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/discovery"
	"k8s.io/client-go/discovery/cached/memory"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/restmapper"

	"main/internal"
)

type turkeyCfg struct {
	Key       string `json:"key"`
	TurkeyId  string `json:"turkeyid"`
	Subdomain string `json:"subdomain"`
	Domain    string `json:"domain"`
	Tier      string `json:"tier"`
	UserEmail string `json:"useremail"`
	DBname    string `json:"dbname"`
	PermsKey  string `json:"permskey"`
	JWK       string `json:"jwk"`
}

var Hc_deploy = http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {

	if r.URL.Path != "/hc_deploy" || r.Method != "POST" {
		http.Error(w, http.StatusText(http.StatusNotFound), http.StatusNotFound)
		return
	}
	sess := internal.GetSession(r.Cookie)

	// #1 prepare configs
	cfg, err := makeCfg(r)
	if err != nil {
		sess.Log("bad turkeyCfg: " + err.Error())
	}

	// #2 render turkey-k8s-chart by apply cfg to turkey.yam
	t, err := template.ParseFiles("./_files/turkey.yam")
	if err != nil {
		sess.Panic(err.Error())
	}
	var buf bytes.Buffer
	t.Execute(&buf, cfg)
	k8sChartYaml := buf.String()

	// #2.5 dry run option
	if cfg.Tier == "dryrun" {
		w.Header().Set("Content-Disposition", "attachment; filename="+cfg.Subdomain+".yaml")
		w.Header().Set("Content-Type", "text/plain")
		io.Copy(w, strings.NewReader(k8sChartYaml))
		return
	}

	// #3 getting k8s config
	sess.Log("&#9989; ... using InClusterConfig")
	k8sCfg, err := rest.InClusterConfig()
	// }
	if k8sCfg == nil {
		sess.Log("ERROR" + err.Error())
		sess.Panic(err.Error())
	}
	sess.Log("&#129311; k8s.k8sCfg.Host == " + k8sCfg.Host)

	// #4 kubectl apply -f <file.yaml> --server-side --field-manager "turkey-userid-<cfg.UserId>"
	sess.Log("&#128640; --- deployment started")
	err = ssa_k8sChartYaml(cfg.TurkeyId, k8sChartYaml, k8sCfg)
	if err != nil {
		sess.Log("ERROR --- deployment FAILED !!! because" + fmt.Sprint(err))
		sess.Panic(err.Error())
	}

	// quality of life improvement for /console people
	skipadminLink := "https://" + cfg.Subdomain + "." + cfg.Domain + "?skipadmin"
	sess.Log("&#128640; --- deployment completed for: <a href=\"" +
		skipadminLink + "\" target=\"_blank\"><b>&#128279;" + cfg.TurkeyId + ":" + cfg.Subdomain + "</b></a>")
	sess.Log("&#128231; --- admin email: " + cfg.UserEmail)

	// #5 create db
	_, err = internal.PgxPool.Exec(context.Background(), "create database \""+cfg.DBname+"\"")
	if err != nil {
		if strings.Contains(err.Error(), "already exists (SQLSTATE 42P04)") {
			sess.Log("db already exists")
			internal.GetLogger().Warn("db <" + cfg.DBname + "> already exists")
			return
		} else {
			sess.Log("ERROR --- DB.conn.Exec FAILED !!! because" + fmt.Sprint(err))
			sess.Panic(err.Error())
		}
	}
	sess.Log("&#128024; --- db created: " + cfg.DBname)

	// // #6 load schema to new db .................. doing it on reticulum boot-up for now, trival perf cost for zero dev impact, good tradeoff until we scaled to millions
	// retSchemaBytes, err := ioutil.ReadFile("./_files/pgSchema.sql")
	// if err != nil {
	// 	sess.Panic(err.Error())
	// }
	// dbconn, err := pgx.Connect(context.Background(), internal.Cfg.DBconn+"/"+cfg.DBname)
	// if err != nil {
	// 	sess.Panic(err.Error())
	// }
	// defer dbconn.Close(context.Background())
	// _, err = dbconn.Exec(context.Background(), string(retSchemaBytes))
	// if err != nil {
	// 	sess.Panic(err.Error())
	// }
	// sess.Log("&#128024; --- schema loaded to db: " + cfg.DBname)

	// #7 done, (todo) return a json report for portal to consume

})

var decUnstructured = yaml.NewDecodingSerializer(unstructured.UnstructuredJSONScheme)

func ssa_k8sChartYaml(userId, k8sChartYaml string, cfg *rest.Config) error {
	// Prepare a RESTMapper to find GVR
	dc, err := discovery.NewDiscoveryClientForConfig(cfg)
	if err != nil {
		return err
	}
	mapper := restmapper.NewDeferredDiscoveryRESTMapper(memory.NewMemCacheClient(dc))
	// Prepare the dynamic client
	dyn, err := dynamic.NewForConfig(cfg)
	if err != nil {
		return err
	}
	for _, k8sYaml := range strings.Split(k8sChartYaml, "\n---\n") {
		// fmt.Println("\n\n\n@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@\n")
		// fmt.Println(k8sYaml)
		// fmt.Println("\n@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@\n\n\n")
		// continue

		// Decode YAML manifest into unstructured.Unstructured
		obj := &unstructured.Unstructured{}
		_, gvk, err := decUnstructured.Decode([]byte(k8sYaml), nil, obj)
		if err != nil {
			return err
		}
		// Find GVR
		mapping, err := mapper.RESTMapping(gvk.GroupKind(), gvk.Version)
		if err != nil {
			return err
		}
		// Obtain REST interface for the GVR
		var dr dynamic.ResourceInterface
		if mapping.Scope.Name() == meta.RESTScopeNameNamespace {
			// namespaced resources should specify the namespace
			dr = dyn.Resource(mapping.Resource).Namespace(obj.GetNamespace())
		} else {
			// for cluster-wide resources
			dr = dyn.Resource(mapping.Resource)
		}
		// Marshal object into JSON
		data, err := json.Marshal(obj)
		if err != nil {
			return err
		}
		// Create or Update the object with SSA // types.ApplyPatchType indicates SSA. // FieldManager specifies the field owner ID.
		_, err = dr.Patch(context.TODO(),
			obj.GetName(), types.ApplyPatchType, data,
			metav1.PatchOptions{
				FieldManager: "turkey-userid-" + userId,
			})
		if err != nil {
			return err
		}
	}
	return err
}

var Hc_get = http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
	if r.URL.Path != "/hc_get" || r.Method != "POST" {
		http.Error(w, http.StatusText(http.StatusNotFound), http.StatusNotFound)
		return
	}
	sess := internal.GetSession(r.Cookie)

	cfg, err := makeCfg(r)
	if err != nil {
		sess.Log("bad turkeyCfg: " + err.Error())
		return
	}

	//<debugging cheatcodes>
	if cfg.TurkeyId[0:4] == "dev_" {
		if cfg.Subdomain != "dev0" {
			fmt.Println("dev_ cheatcodes only work with subdomain == dev0 ")
			return
		}
		sess.Log(`turkeyUserId[0:4] == dev_ means dev mode`)

		cfg.UserEmail = "gtan@mozilla.com"
		t, _ := template.ParseFiles("./_files/turkey.yam")
		var buf bytes.Buffer
		t.Execute(&buf, cfg)
		k8sChartYaml := buf.String()
		if cfg.TurkeyId == "dev_dumpr" {
			sess.Log(dumpHeader(r))
			return
		}
		if cfg.TurkeyId == "dev_gimmechart" {
			w.Header().Set("Content-Disposition", "attachment; filename="+cfg.Subdomain+".yaml")
			w.Header().Set("Content-Type", "text/plain")
			io.Copy(w, strings.NewReader(k8sChartYaml))
			return
		}
	}
	//</debugging cheatcodes>

	//getting k8s config
	sess.Log("&#9989; ... using InClusterConfig")
	k8sCfg, err := rest.InClusterConfig()
	// }
	if k8sCfg == nil {
		sess.Log("ERROR" + err.Error())
		sess.Panic(err.Error())
	}
	sess.Log("&#129311; k8s.k8sCfg.Host == " + k8sCfg.Host)
	clientset, err := kubernetes.NewForConfig(k8sCfg)
	if err != nil {
		sess.Panic(err.Error())
	}
	//list ns
	nsList, err := clientset.CoreV1().Namespaces().List(context.TODO(),
		metav1.ListOptions{
			LabelSelector: "TurkeyId=" + cfg.TurkeyId,
		})
	if err != nil {
		sess.Panic(err.Error())
	}
	sess.Log("GET --- user <" + cfg.TurkeyId + "> owns: ")
	for _, ns := range nsList.Items {
		sess.Log("......GetName: <" + ns.ObjectMeta.GetName() + ">")
		sess.Log("......GetNamespace: <" + ns.ObjectMeta.GetNamespace() + ">")
		sess.Log("......dump: <" + fmt.Sprint(ns.ObjectMeta) + ">")
	}
})

func makeCfg(r *http.Request) (turkeyCfg, error) {
	var cfg turkeyCfg

	//get r.body
	rBodyBytes, err := ioutil.ReadAll(r.Body)
	if err != nil {
		fmt.Println("ERROR @ reading r.body, error = " + err.Error())
		return cfg, err
	}
	//make cfg
	err = json.Unmarshal(rBodyBytes, &cfg)
	if err != nil {
		fmt.Println("bad turkeyCfg: " + string(rBodyBytes))
		return cfg, err
	}

	// userid is required
	if cfg.TurkeyId == "" {
		return cfg, errors.New("ERROR bad turkeyCfg.UserId")
	}
	cfg.Domain = internal.Cfg.Domain
	//default Tier is free
	if cfg.Tier == "" {
		cfg.Tier = "free"
	}
	//default Subdomain is a string hashed from turkeyId and time
	if cfg.Subdomain == "" {
		cfg.Subdomain = cfg.TurkeyId + "-" + strconv.FormatInt(time.Now().Unix()-1626102245, 36)
	}
	cfg.DBname = "ret_" + strings.ReplaceAll(cfg.Subdomain, "-", "_")
	//use authenticated useremail
	cfg.UserEmail = r.Header.Get("X-Forwarded-UserEmail")
	if cfg.UserEmail == "" {
		// return cfg, errors.New("failed to get cfg.UserEmail")
		cfg.UserEmail = "fooooo@barrrr.com"
	}

	//cluster wide private key for all reticulum authentications
	permskey_in := os.Getenv("PERMS_KEY")
	if permskey_in == "" {
		return cfg, errors.New("bad perms_key")
	}
	cfg.PermsKey = strings.ReplaceAll(permskey_in, `\n`, `\\n`)
	perms_key_str := strings.Replace(permskey_in, `\n`, "\n", -1)
	pb, _ := pem.Decode([]byte(perms_key_str))
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

	return cfg, nil
}

var Hc_delDB = http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
	if r.URL.Path != "/hc_delDB" || r.Method != "POST" {
		http.Error(w, http.StatusText(http.StatusNotFound), http.StatusNotFound)
		return
	}
	sess := internal.GetSession(r.Cookie)
	cfg, err := makeCfg(r)
	if err != nil {
		sess.Panic("bad turkeyCfg: " + err.Error())
		return
	}

	sess.Log("&#128024 deleting db: " + cfg.DBname)

	//delete db -- force
	force := true
	_, err = internal.PgxPool.Exec(context.Background(), "drop database "+cfg.DBname)
	if err != nil {
		if strings.Contains(err.Error(), "is being accessed by other users (SQLSTATE 55006)") && force {
			err = pg_kick_all(cfg, sess)
			if err != nil {
				sess.Panic(err.Error())
			}
			_, err = internal.PgxPool.Exec(context.Background(), "drop database "+cfg.DBname)
		}
		if err != nil {
			sess.Panic(err.Error())
		}
	}
	sess.Log("&#128024 deleted db: " + cfg.DBname)
})

func pg_kick_all(cfg turkeyCfg, sess *internal.CacheBoxSessData) error {
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

var Hc_delNS = http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
	if r.URL.Path != "/hc_delNS" || r.Method != "POST" {
		http.Error(w, http.StatusText(http.StatusNotFound), http.StatusNotFound)
		return
	}
	sess := internal.GetSession(r.Cookie)
	cfg, err := makeCfg(r)
	if err != nil {
		sess.Log("bad turkeyCfg: " + err.Error())
		return
	}

	//getting k8s config
	sess.Log("&#9989; ... using InClusterConfig")
	k8sCfg, err := rest.InClusterConfig()
	// }
	if k8sCfg == nil {
		sess.Panic(err.Error())
	}
	sess.Log("&#129311; k8s.k8sCfg.Host == " + k8sCfg.Host)
	clientset, err := kubernetes.NewForConfig(k8sCfg)
	if err != nil {
		sess.Panic("failed to get kubecfg" + err.Error())
	}

	//delete ns
	nsName := "hc-" + cfg.Subdomain
	err = clientset.CoreV1().Namespaces().Delete(context.TODO(),
		nsName,
		metav1.DeleteOptions{})
	if err != nil {
		sess.Panic("delete ns failed: " + err.Error())
	}
	sess.Log("&#127754 deleted ns: " + nsName)

})
