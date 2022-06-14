package main

import (
	"bytes"
	"crypto/tls"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/google/uuid"
)

var (
	turkeyDomain      = "gtan.myhubs.net"
	_turkeyauthcookie = "ou-r9IGIe1YPs-Fxt2NnaO13wu8HUNzHpVaYmpIBeh0=|1655257835|gtan@mozilla.com"

	useremail = "gtan@mozilla.com"

	_httpClient = &http.Client{
		Timeout:   10 * time.Second,
		Transport: &http.Transport{TLSClientConfig: &tls.Config{InsecureSkipVerify: true}},
	}
)

func main() {

	vu := &vuser{
		hubId: "tb" + strings.ReplaceAll(uuid.New().String(), "-", ""),
	}
	vu.create()
	vu.load()
	vu.delete()
	fmt.Printf("\n vu: %v", vu)
}

type vuser struct {
	hubId   string
	tCreate time.Duration
	tReady  time.Duration
}

func (vu *vuser) create() {

	//create
	createReqBody, _ := json.Marshal(map[string]string{
		"hub_id":    vu.hubId,
		"useremail": useremail,
	})
	createReq, err := http.NewRequest("POST", "https://orch."+turkeyDomain+"/hc_instance", bytes.NewBuffer(createReqBody))
	createReq.AddCookie(&http.Cookie{
		Name:  "_turkeyauthcookie",
		Value: _turkeyauthcookie,
	})
	if err != nil {
		panic("createReq error: " + err.Error())
	}
	tStart := time.Now()
	resp, err := _httpClient.Do(createReq)
	if err != nil && resp.StatusCode != http.StatusOK {
		fmt.Printf("err: %v, resp: %v", err, resp)
		panic("failed @ creation")
	}
	vu.tCreate = time.Since(tStart)
	fmt.Printf("\n[tCreate: %v] for hubId %v", vu.tCreate, vu.hubId)

	//wait for ret
	tStart = time.Now()
	timeout := time.Now().Add(5 * time.Minute)
	retReq, _ := http.NewRequest("GET", "https://"+vu.hubId+"."+turkeyDomain+"/?skipadmin", nil)
	resp, err = _httpClient.Do(retReq)
	for err != nil || resp.StatusCode != http.StatusOK {
		time.Sleep(5 * time.Second)
		ttl := time.Until(timeout)
		if resp != nil {
			fmt.Printf("\ngot: %v, retrying, ttl: %v, hubId: %v, req: %v", resp.StatusCode, ttl, vu.hubId, retReq.URL)
			bodyBytes, _ := io.ReadAll(resp.Body)
			fmt.Printf("\nresq: %v", string(bodyBytes))
		} else {
			fmt.Printf("\ngot: %v, retrying, ttl: %v, hubId: %v, req: %v", err, ttl, vu.hubId, retReq.URL)
		}
		if ttl < 0 {
			fmt.Printf("\nerr: timeout -- hubId %v", vu.hubId)
			vu.tReady = -1
		}
		resp, err = _httpClient.Do(retReq)
	}
	if vu.tReady != -1 {
		vu.tReady = time.Since(tStart)
	}
	fmt.Printf("[tReady: %v] for hubId %v", vu.tCreate, vu.hubId)
}

func (vu *vuser) delete() {
	//delete
	deleteReqBody, _ := json.Marshal(map[string]string{
		"hub_id": vu.hubId,
	})
	deleteReq, err := http.NewRequest("DELETE", "https://orch."+turkeyDomain+"/hc_instance", bytes.NewBuffer(deleteReqBody))
	deleteReq.AddCookie(&http.Cookie{
		Name:  "_turkeyauthcookie",
		Value: _turkeyauthcookie,
	})
	if err != nil {
		panic("deleteReq error: " + err.Error())
	}
	resp, err := _httpClient.Do(deleteReq)
	if err != nil && resp.StatusCode != http.StatusOK {
		fmt.Printf("\nerr: %v, resp: %v", err, resp)
		panic("failed @ delete")
	}
	fmt.Printf("\n[deleted: %v]", vu.hubId)
}

func (vu *vuser) load() {
	for i := 1; i <= 5; i++ {
		time.Sleep(1 * time.Second)
		fmt.Printf(". ")
	}
}
