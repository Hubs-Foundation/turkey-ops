package handlers

import (
	"encoding/json"
	"fmt"
	"main/internal"
	"net/http"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/service/lambda"
)

var Ytdl = http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
	if r.URL.Path != "/ytdl/api/info" {
		http.Error(w, http.StatusText(http.StatusNotFound), http.StatusNotFound)
		return
	}
	fmt.Println("RequestURI: " + r.RequestURI)
	// internal.GetLogger().Debug(dumpHeader(r))
	fmt.Println("r.URL.RawQuery:", r.URL.RawQuery)
	fmt.Println("headers: ", dumpHeader(r))

	lambdaEvent := map[string]string{"url": "http://whatever/?url=https://www.youtube.com/watch?v=zjMuIxRvygQ&moreparams=values"}

	payload, _ := json.Marshal(lambdaEvent)

	lambdaClient := lambda.New(internal.Cfg.Awss.Sess)
	resp, err := lambdaClient.Invoke(
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
	// fmt.Fprint(w, string(resp.Payload))
	// json.NewEncoder(w).Encode(string(resp.Payload))
	// http.Error(w, "comming soon", http.StatusNotImplemented)
	// fmt.Println(string(resp.Payload))
	return
})
