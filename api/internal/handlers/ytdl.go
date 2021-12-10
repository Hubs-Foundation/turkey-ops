package handlers

import (
	"encoding/json"
	"main/internal"
	"net/http"
	"net/url"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/service/lambda"
)

var Ytdl = http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
	if r.URL.Path != "/ytdl/api/info" {
		http.Error(w, http.StatusText(http.StatusNotFound), http.StatusNotFound)
		return
	}
	query, err := url.QueryUnescape(r.URL.RawQuery)
	if err != nil {
		internal.GetLogger().Panic("failed to unescape r.URL.RawQuery: " + r.URL.RawQuery)
	}

	payload, _ := json.Marshal(map[string]string{"url": "asdf?" + query})
	resp, err := lambda.New(internal.Cfg.Awss.Sess).Invoke(
		&lambda.InvokeInput{
			FunctionName: aws.String("dev_ytdl_001"),
			Payload:      payload,
		},
	)
	if err != nil {
		internal.GetLogger().Panic("failed to invoke lambda: " + err.Error())
	}

	if resp.FunctionError != nil {
		internal.GetLogger().Panic("ytdl lambda failed: " + *resp.FunctionError)
	}
	w.Header().Set("Content-Type", "application/json")
	w.Write(resp.Payload)

	var m map[string]string
	json.Unmarshal(resp.Payload, &m)
	internal.GetLogger().Debug("ytdl query: " + query)
	internal.GetLogger().Debug("ytdl debugMsg: " + m["debugMsg"])

	// fmt.Fprint(w, string(resp.Payload))
	// json.NewEncoder(w).Encode(string(resp.Payload))
	// http.Error(w, "comming soon", http.StatusNotImplemented)
	// fmt.Println(string(resp.Payload))
})
