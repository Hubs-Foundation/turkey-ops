package internal

import (
	"bytes"
	"crypto/tls"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

var (
	turkeyDomain      = "gtan.myhubs.net"
	_turkeyauthcookie = "WmeluLWXgwZ_U3Vk6k073-dtT9jNCd1WJ6_7vRkhKh4=|1655339779|gtan@mozilla.com"

	useremail = "gtan@mozilla.com"

	_httpClient = &http.Client{
		Timeout:   10 * time.Second,
		Transport: &http.Transport{TLSClientConfig: &tls.Config{InsecureSkipVerify: true}},
	}
)

type Vuser struct {
	HubId   string
	TCreate time.Duration
	TReady  time.Duration
}

func (vu *Vuser) Create() {
	fmt.Printf("\n######creating######, vu: %v", vu.HubId)

	//create
	createReqBody, _ := json.Marshal(map[string]string{
		"hub_id":    vu.HubId,
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
	bodyBytes, _ := io.ReadAll(resp.Body)
	if err != nil && resp.StatusCode != http.StatusOK && strings.Contains(string(bodyBytes), `"result":"done"`) {
		fmt.Printf("err: %v, resp: %v, resp.body: %v", err, resp, string(bodyBytes))
		panic("failed @ creation")
	}
	fmt.Printf("\n---creation succeeded")

	vu.TCreate = time.Since(tStart)
	fmt.Printf("\n[tCreate: %v] for hubId %v", vu.TCreate, vu.HubId)

	//wait for ret
	tStart = time.Now()
	timeout := time.Now().Add(5 * time.Minute)
	retReq, _ := http.NewRequest("GET", "https://"+vu.HubId+"."+turkeyDomain+"/?skipadmin", nil)
	fmt.Printf("\nreq: %v", retReq.URL)
	resp, err = _httpClient.Do(retReq)
	for err != nil || resp.StatusCode != http.StatusOK {
		time.Sleep(5 * time.Second)
		ttl := time.Until(timeout)
		if resp != nil {
			// bodyBytes, _ := io.ReadAll(resp.Body)
			// fmt.Printf("\n---[waiting-for-ret] got: %v[%v], retrying, ttl: %v, hubId: %v", resp.StatusCode, string(bodyBytes), ttl, vu.HubId)
			fmt.Printf("\n---[waiting-for-ret] got: %v, retrying, ttl: %v, hubId: %v", resp.StatusCode, ttl, vu.HubId)
		} else {
			fmt.Printf("\n---[waiting-for-ret]: %v, retrying, ttl: %v, hubId: %v", err, ttl, vu.HubId)
		}
		if ttl < 0 {
			fmt.Printf("\n---[waiting-for-ret]:err: timeout -- hubId %v", vu.HubId)
			vu.TReady = -1
		}
		resp, err = _httpClient.Do(retReq)
	}
	if vu.TReady != -1 {
		vu.TReady = time.Since(tStart)
	}
	fmt.Printf("[tReady: %v] for hubId %v", vu.TCreate, vu.HubId)
}

func (vu *Vuser) Delete() {
	fmt.Printf("\n######deleting######, vu: %v", vu.HubId)
	//delete
	deleteReqBody, _ := json.Marshal(map[string]string{
		"hub_id": vu.HubId,
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
	fmt.Printf("\n[deleted: %v]", vu.HubId)
}

func (vu *Vuser) Load() {
	fmt.Printf("\n######loading <fake ... real one with bots coming soon> ######, vu: %v", vu.HubId)
	for i := 1; i <= 10; i++ {
		time.Sleep(1 * time.Second)
		fmt.Printf(". ")
	}
	fmt.Printf("done")
}

func (vu *Vuser) ToString() string {
	return fmt.Sprintf(`{ vu.HubId: %v, vu.tCreate: %v, vu.tReady: %v}`, vu.HubId, vu.TCreate, vu.TReady)
}
