package handlers

import (
	"encoding/json"
	"io/ioutil"
	"main/internal"
	"net/http"
)

var Webhook_peerReport = http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
	if r.Method == "POST" {
		handlePeerReport(r)
	}
	http.Error(w, http.StatusText(http.StatusNotFound), http.StatusNotFound)
})

func handlePeerReport(r *http.Request) {

	internal.Logger.Sugar().Debugf("r.RequestURI: %v", r.RequestURI)

	rBodyBytes, err := ioutil.ReadAll(r.Body)
	if err != nil {
		internal.Logger.Error("ERROR @ reading r.body, error = " + err.Error())
		return
	}
	report := internal.PeerReport{}

	err = json.Unmarshal(rBodyBytes, &report)
	if err != nil {
		internal.Logger.Sugar().Errorf("failed to unmarshalbad (%v), err: %v", string(rBodyBytes), err)
		return
	}
	internal.Logger.Sugar().Debugf("report: %v", report)

	internal.Cfg.PeerMan.UpdatePeerAndUpload(report)

}
