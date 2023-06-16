package handlers

import (
	"bytes"
	"encoding/json"
	"errors"
	"io"
	"main/internal"
	"net/http"
)

func handleMultiClusterReq(w http.ResponseWriter, r *http.Request, cfg HCcfg) error {

	// tStart := time.Now()

	internal.Logger.Debug("multi-cluster req, hcCfg.Region: " + cfg.Region)

	cfg.TurkeyJobReqMethod = r.Method
	cfg.TurkeyJobJobId = w.Header().Get("X-Request-Id")
	cfg.TurkeyJobCallback = internal.Cfg.TurkeyJobCallback

	// // gcp-pubsub option, step 1: publish message
	// cfgBytes, _ := json.Marshal(cfg)
	// err := internal.Cfg.Gcps.PubSub_PublishMsg(internal.Cfg.TurkeyJobsPubSubTopicName, cfgBytes)
	// if err != nil {
	// 	return err
	// }
	// //gcp-pubsub option, step 2: wait for callback
	// callback_arr, err := internal.Cfg.Redis.BLPop(60*time.Second, cfg.TurkeyJobJobId)
	// if err != nil {
	// 	internal.Logger.Sugar().Debugf("failed @ catching callback: %v", err)
	// }
	// internal.Logger.Sugar().Debugf("callback_arr: %v", callback_arr)

	// for i, e := range callback_arr {
	// 	fmt.Println("i:", i, ", e:", e)
	// }

	// resultMap := map[string]string{}
	// err = json.Unmarshal([]byte(callback_arr[1]), &resultMap)
	// if err != nil {
	// 	internal.Logger.Sugar().Errorf("err: %v", err)
	// }

	// root-cluter-proxy option, step1: locate the peer cluster
	peers := internal.Cfg.PeerMan.FindPeerDomain(cfg.Region)
	if len(peers) == 0 {
		internal.Logger.Sugar().Errorf(
			"no appropriate peer for region: %v (new regional peer cluster are manually created atm)", cfg.Region)
		return errors.New("no appropriate peer for region: " + cfg.Region)
	}
	internal.Logger.Sugar().Debugf("located peers: %v", peers)

	done := false
	pick := 0
	resultMap := map[string]string{}

	for !done && pick < len(peers)-1 {
		peerDomain := peers[pick].Domain
		peerToken := peers[pick].Token
		pick++

		jsonPayload, _ := json.Marshal(cfg)
		peerOrchWebhook := "https://orch." + peerDomain + "/webhooks/remote_hc_instance"
		hcReq, _ := http.NewRequest(cfg.TurkeyJobReqMethod, peerOrchWebhook, bytes.NewBuffer(jsonPayload))
		hcReq.Header.Add("token", peerToken)
		// internal.Logger.Sugar().Debugf("hcReq: %v", hcReq)
		resp, err := http.DefaultClient.Do(hcReq)
		if err != nil {
			internal.Logger.Sugar().Errorf("failed to sed out hcReq: %v", err)
		}
		if resp.StatusCode < 300 {
			done = true
		} else {
			respBodyBytes, _ := io.ReadAll(resp.Body)
			internal.Logger.Sugar().Warnf(
				"failed -- domain: <%v>, resp.code:<%v>, resp.body:<%v>", peerDomain, resp.StatusCode, string(respBodyBytes))
			continue
		}
		respBodyBytes, _ := io.ReadAll(resp.Body)
		err = json.Unmarshal(respBodyBytes, &resultMap)
		if err != nil {
			internal.Logger.Sugar().Errorf("err: %v (respBody: %v)", err, string(respBodyBytes))
		}
	}
	if !done {
		internal.Logger.Sugar().Errorf("failed on all peer clusters. %v", peers)
	}

	//-----------------------------------------------------------
	// resultMap produced
	internal.Logger.Sugar().Debugf("resultMap: %v", resultMap)
	// tElapsed := time.Since(tStart)
	json.NewEncoder(w).Encode(resultMap)
	return nil
}

var Dump_peerMap = http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {

	peerMap := internal.Cfg.PeerMan.GetPeerMap()

	json.NewEncoder(w).Encode(peerMap)

})
