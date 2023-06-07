package internal

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"image"
	"image/png"
	"io/ioutil"
	"math/rand"
	"net/http"
	"os"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/gorilla/websocket"

	corev1 "k8s.io/api/core/v1"
	networkingv1 "k8s.io/api/networking/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

var captchaSolve = int32(111)

var Root_Pausing = http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {

	if strings.HasSuffix(r.URL.Path, "/websocket") {
		upgrader := websocket.Upgrader{
			ReadBufferSize:  1024,
			WriteBufferSize: 1024,
		}
		conn, err := upgrader.Upgrade(w, r, nil)
		if err != nil {
			Logger.Error("failed to upgrade: " + err.Error())
			http.Error(w, "", http.StatusInternalServerError)
			return
		}
		defer conn.Close()
		var tResumeStart time.Time
		go func() {
			// rand.Seed(time.Now().UnixNano())

			//status report during HC_Resume(), incl cooldown period
			for {
				Logger.Sugar().Debugf("_resuming_status: %v", _resuming_status)
				time.Sleep(3 * time.Second)
				if _resuming_status == 0 {
					continue
				}
				time.Sleep(3 * time.Second)

				sendMsg := fmt.Sprintf("cooldown in progress -- try again in %v min", (_resuming_status / 60))
				if _resuming_status < 0 {
					sendMsg = fmt.Sprintf("waiting for backends...(%v)", time.Since(tResumeStart).Seconds())
				}
				if float64(_resuming_status) > (cfg.FreeTierIdleMax.Seconds()*1.25 - 60) {
					sendMsg = "_refresh_"
				}
				Logger.Debug("sendMsg: " + sendMsg)
				err := conn.WriteMessage(websocket.TextMessage, []byte(sendMsg))
				if err != nil {
					Logger.Debug("err @ conn.WriteMessage:" + err.Error())
					break
				}
			}
		}()

		for {
			mt, message, err := conn.ReadMessage()
			if err != nil {
				Logger.Debug("err @ conn.ReadMessage:" + err.Error())
				break
			}
			strMessage := string(message)
			Logger.Sugar().Debugf("recv: type=<%v>, msg=<%v>", mt, string(strMessage))
			if strMessage == "hi" {
				conn.WriteMessage(websocket.TextMessage, []byte("hi"))
			}
			if strings.HasPrefix(strMessage, "_r_:") && _resuming_status == 0 {
				tResumeStart = time.Now()
				HC_Resume()
				err = conn.WriteMessage(mt, []byte("respawning hubs pods"))
				if err != nil {
					Logger.Debug("err @ conn.WriteMessage:" + err.Error())
					break
				}
			}
		}
	}

	switch r.Method {
	case "GET":
		if _, ok := r.Header["Duck"]; ok {
			duckpng, _ := os.Open("./_statics/duck.png")
			defer duckpng.Close()
			img, _, _ := image.Decode(duckpng)
			rand.Seed(time.Now().UnixNano())
			rotatedImg := rotateImg(img, 25+rand.Float64()*250)
			var buffer bytes.Buffer
			_ = png.Encode(&buffer, rotatedImg)
			encoded := base64.StdEncoding.EncodeToString(buffer.Bytes())
			fmt.Fprint(w, encoded)
			return
		}
		bytes, err := ioutil.ReadFile("./_statics/pausing.html")
		if err != nil {
			Logger.Sugar().Errorf("%v", err)
		}
		fmt.Fprint(w, string(bytes))
	case "PUT":
		if _resuming_status != 0 {
			http.Error(w, "", http.StatusBadRequest)
			return
		}
		err := HC_Resume()
		if err != nil {
			Logger.Sugar().Errorf("err (reqId: %v): %v", w.Header().Get("X-Request-Id"), err)
			fmt.Fprintf(w, "something went wrong -- reqId=%v", w.Header().Get("X-Request-Id"))
			return
		}
		fmt.Fprint(w, "resuming")
	case "UPDATE":
		Logger.Sugar().Debugf("_resuming_status: %v", _resuming_status)
		if _resuming_status == 0 {
			fmt.Fprint(w, "slide to fix the ducks orintation")
			return
		}
		if _resuming_status < 0 {
			fmt.Fprintf(w, "resuming, this can take a few minutes")
			return
		}
		fmt.Fprintf(w, "not ready yet, try again in %v min", (_resuming_status / 60))
	default:
		http.Error(w, "", 404)
	}
})

var Z_Pause = http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
	err := HC_Pause()
	fmt.Fprintf(w, "err: %v", err)
})

var Z_Resume = http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
	if _resuming_status != 0 {
		http.Error(w, string(_resuming_status), http.StatusBadRequest)
		return
	}
	err := HC_Resume()
	if err != nil {
		Logger.Sugar().Errorf("err (reqId: %v): %v", w.Header().Get("X-Request-Id"), err)
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	fmt.Fprint(w, "started")
})

func HC_Pause() error {

	//back up current ingresses
	igs, err := cfg.K8sClientSet.NetworkingV1().Ingresses(cfg.PodNS).List(context.Background(), metav1.ListOptions{})
	if err != nil {
		Logger.Error("failed to get ingresses: " + err.Error())
		return errors.New("failed to get ingress" + err.Error())
	}
	if len(igs.Items) < 2 || igs.Items[0].Name == "pausing" {
		return fmt.Errorf("UNEXPECTED VERY BAD EXCEPTION -- igs: %v", igs)
	}

	igsbak, err := json.Marshal(*igs)
	if err != nil {
		Logger.Error("failed to marshal ingresses: " + err.Error())
	}

	igsbak_cm := &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{Name: "igsbak"},
		BinaryData: map[string][]byte{"igsbak": igsbak},
	}
	_, err = cfg.K8sClientSet.CoreV1().ConfigMaps(cfg.PodNS).Create(context.Background(), igsbak_cm, metav1.CreateOptions{})
	if err != nil {
		_, err = cfg.K8sClientSet.CoreV1().ConfigMaps(cfg.PodNS).Update(context.Background(), igsbak_cm, metav1.UpdateOptions{})
		if err != nil {
			Logger.Error("failed to create/update ig_bak configmap:" + err.Error())
		}
	}

	//delete current ingresses
	for _, ig := range igs.Items {
		err := cfg.K8sClientSet.NetworkingV1().Ingresses(cfg.PodNS).Delete(context.Background(), ig.Name, metav1.DeleteOptions{})
		if err != nil {
			Logger.Error("failed to delete ingresses" + err.Error())
		}
	}
	//create pausing ingress
	// pathType_exact := networkingv1.PathTypeExact
	pathType_prefix := networkingv1.PathTypePrefix
	_, err = cfg.K8sClientSet.NetworkingV1().Ingresses(cfg.PodNS).Create(context.Background(),
		&networkingv1.Ingress{
			ObjectMeta: metav1.ObjectMeta{
				Name:        "pausing",
				Annotations: map[string]string{"kubernetes.io/ingress.class": "haproxy"},
			},
			Spec: networkingv1.IngressSpec{
				Rules: []networkingv1.IngressRule{
					{
						Host: cfg.SubDomain + "." + cfg.HubDomain,
						IngressRuleValue: networkingv1.IngressRuleValue{
							HTTP: &networkingv1.HTTPIngressRuleValue{
								Paths: []networkingv1.HTTPIngressPath{
									{
										Path:     "/",
										PathType: &pathType_prefix,
										Backend: networkingv1.IngressBackend{
											Service: &networkingv1.IngressServiceBackend{
												Name: "ita",
												Port: networkingv1.ServiceBackendPort{
													Number: 6000,
												}}}},
								}}}}}},
		}, metav1.CreateOptions{})
	if err != nil {
		Logger.Error("failed to create pausing ingresses" + err.Error())
	}

	// scale down deployments, except ita
	ds, err := cfg.K8sClientSet.AppsV1().Deployments(cfg.PodNS).List(context.Background(), metav1.ListOptions{})
	if err != nil {
		Logger.Error("failed to list deployments: " + err.Error())
		return err
	}
	for _, d := range ds.Items {
		if d.Name == "ita" {
			continue
		}
		d.Spec.Replicas = pointerOfInt32(0)
		_, err := cfg.K8sClientSet.AppsV1().Deployments(cfg.PodNS).Update(context.Background(), &d, metav1.UpdateOptions{})
		if err != nil {
			Logger.Sugar().Errorf("failed to scale down %v: %v"+d.Name, err.Error())
			return err
		}
	}

	NS_setLabel("paused", "1")

	return nil
}

var _resuming_status = int32(0)
var reMu sync.Mutex

func HC_Resume() error {

	reMu.Lock()
	if _resuming_status != 0 {
		return nil
	}
	atomic.StoreInt32(&_resuming_status, -1)
	reMu.Unlock()

	Logger.Sugar().Debugf("resuming in progress, _resuming_status=%v", _resuming_status)
	// scale back deployments
	ds, err := cfg.K8sClientSet.AppsV1().Deployments(cfg.PodNS).List(context.Background(), metav1.ListOptions{})
	if err != nil {
		Logger.Error("failed to list deployments: " + err.Error())
		return err
	}
	for _, d := range ds.Items {
		if d.Name == "ita" {
			continue
		}
		d.Spec.Replicas = pointerOfInt32(1)
		_, err := cfg.K8sClientSet.AppsV1().Deployments(cfg.PodNS).Update(context.Background(), &d, metav1.UpdateOptions{})
		if err != nil {
			Logger.Sugar().Errorf("failed to scale back %v: %v"+d.Name, err.Error())
			return err
		}
	}
	go func() {
		//wait for ret pod
		ret_readyReplicaCnt := 0
		ttl := 5 * time.Minute
		for ret_readyReplicaCnt < 1 && ttl > 0 {
			ret_d, err := cfg.K8sClientSet.AppsV1().Deployments(cfg.PodNS).Get(context.Background(), "reticulum", metav1.GetOptions{})
			if err != nil {
				Logger.Sugar().Errorf("failed to get reticulum deployment in ns %v", cfg.PodNS)
				time.Sleep(5 * time.Second)
				continue
			}
			ret_readyReplicaCnt = int(ret_d.Status.ReadyReplicas)
			Logger.Sugar().Debugf("waiting for ret, ttl: %v", ttl)
			time.Sleep(30 * time.Second)
			ttl -= 30 * time.Second
		}
		Logger.Debug("ret's ready")

		// delete pausing ingress
		err = cfg.K8sClientSet.NetworkingV1().Ingresses(cfg.PodNS).Delete(context.Background(), "pausing", metav1.DeleteOptions{})
		if err != nil {
			Logger.Error("failed to delete pausing ingresses" + err.Error())
		}

		//restore ig_bak
		igsbak_cm, err := cfg.K8sClientSet.CoreV1().ConfigMaps(cfg.PodNS).Get(context.Background(), "igsbak", metav1.GetOptions{})
		if err != nil {
			Logger.Error("failed to get ig_bak configmap:" + err.Error())
		}
		igsbak := igsbak_cm.BinaryData["igsbak"]
		var igs networkingv1.IngressList
		err = json.Unmarshal(igsbak, &igs)
		if err != nil {
			Logger.Sugar().Errorf("failed to unmarshal igsbak: %v", err)
		}
		for _, ig := range igs.Items {
			ig.ResourceVersion = ""
			_, err := cfg.K8sClientSet.NetworkingV1().Ingresses(cfg.PodNS).Create(context.Background(), &ig, metav1.CreateOptions{})
			if err != nil {
				Logger.Sugar().Errorf("failed to restore ig_bak: %v", err)
			}
		}

		cooldown := cfg.FreeTierIdleMax.Seconds() * 1.25
		for cooldown > 0 {
			time.Sleep(11 * time.Second)
			cooldown -= 11
			atomic.StoreInt32(&_resuming_status, int32(cooldown))
		}
		atomic.StoreInt32(&_resuming_status, 0)
		NS_setLabel("paused", "0")

	}()

	return nil
}
