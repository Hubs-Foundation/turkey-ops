package handlers

import (
	"encoding/json"
	"io/ioutil"
	"main/internal"
	"net/http"
	"time"
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
		internal.Logger.Sugar().Errorf("failed to unmarshal (%v), err: %v", string(rBodyBytes), err)
		return
	}
	tReported, _ := time.Parse(internal.CONST_DEFAULT_TIME_FORMAT, report.TimeStamp)
	internal.Logger.Sugar().Debugf("report: %v, timediff(why so big?): %v", report, time.Since(tReported))

	//reset TimeStamp because (again why)timediff ^
	report.TimeStamp = time.Now().Format(internal.CONST_DEFAULT_TIME_FORMAT)

	internal.Cfg.PeerMan.UpdatePeerAndUpload(report)

}
