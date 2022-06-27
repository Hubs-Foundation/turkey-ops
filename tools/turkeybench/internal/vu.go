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

var _httpClient = &http.Client{
	Timeout:   100 * time.Second,
	Transport: &http.Transport{TLSClientConfig: &tls.Config{InsecureSkipVerify: true}},
}

type Vuser struct {
	turkeyDomain      string //	ie. "gtan.myhubs.net"
	_turkeyauthcookie string // ie. "WmeluLWXgwZ_U3Vk6k073-dtT9jNCd1WJ6_7vRkhKh4=|1655339779|gtan@mozilla.com"
	useremail         string // 	ie. "gtan@mozilla.com"

	Id      string
	HubId   string
	TCreate time.Duration
	TReady  time.Duration
	Url     string
}

func NewVuser(Id, domain, authcookie, email, hubId string) *Vuser {
	return &Vuser{
		turkeyDomain:      domain,
		_turkeyauthcookie: authcookie,
		useremail:         email,
		HubId:             hubId,
		Url:               "https://" + hubId + "." + domain + "/?skipadmin",
		Id:                Id,
	}
}

func (vu *Vuser) Create() {
	//create
	createReqBody, _ := json.Marshal(map[string]string{
		"hub_id":    vu.HubId,
		"useremail": vu.useremail,
	})
	createReq, err := http.NewRequest("POST", "https://orch."+vu.turkeyDomain+"/hc_instance", bytes.NewBuffer(createReqBody))
	createReq.AddCookie(&http.Cookie{
		Name:  "_turkeyauthcookie",
		Value: vu._turkeyauthcookie,
	})
	if err != nil {
		panic("createReq error: " + err.Error())
	}

	tStart := time.Now()
	resp, err := _httpClient.Do(createReq)
	if err != nil {
		panic(err)
	}
	bodyBytes, _ := io.ReadAll(resp.Body)
	if err != nil && resp.StatusCode != http.StatusOK && strings.Contains(string(bodyBytes), `"result":"done"`) {
		fmt.Printf("err: %v, resp: %v, resp.body: %v", err, resp, string(bodyBytes))
		panic("failed @ creation")
	}
	vu.TCreate = time.Since(tStart)
	fmt.Printf("\n[%v] -- [tCreate: %v]", vu.Id, vu.TCreate)
	//wait for ret
	tStart = time.Now()
	timeout := time.Now().Add(15 * time.Minute)
	retReq, _ := http.NewRequest("GET", "https://"+vu.HubId+"."+vu.turkeyDomain+"/?skipadmin", nil)
	// fmt.Printf("\nreq: %v", retReq.URL)
	resp, err = _httpClient.Do(retReq)
	for err != nil || resp.StatusCode != http.StatusOK {
		time.Sleep(5 * time.Second)
		ttl := time.Until(timeout)
		// if resp != nil {
		// 	// bodyBytes, _ := io.ReadAll(resp.Body)
		// 	// fmt.Printf("\n---[waiting-for-ret] got: %v[%v], retrying, ttl: %v, hubId: %v", resp.StatusCode, string(bodyBytes), ttl, vu.HubId)
		// 	fmt.Printf("\n---[waiting-for-ret] got: %v, retrying, ttl: %v, hubId: %v", resp.StatusCode, ttl, vu.HubId)
		// } else {
		// 	fmt.Printf("\n---[waiting-for-ret]: %v, retrying, ttl: %v, hubId: %v", err, ttl, vu.HubId)
		// }
		if ttl < 0 {
			fmt.Printf("\n---[waiting-for-ret]:err: timeout -- hubId %v", vu.HubId)
			vu.TReady = -1
		}
		resp, err = _httpClient.Do(retReq)
	}
	if vu.TReady != -1 {
		vu.TReady = time.Since(tStart)
	}
	fmt.Printf("\n [%v] [tReady: %v] @ %v", vu.Id, vu.TReady, vu.Url)
}

func (vu *Vuser) Delete() {
	fmt.Printf("\n[deleting: %v]", vu.HubId)
	//delete
	deleteReqBody, _ := json.Marshal(map[string]string{
		"hub_id": vu.HubId,
	})
	deleteReq, err := http.NewRequest("DELETE", "https://orch."+vu.turkeyDomain+"/hc_instance", bytes.NewBuffer(deleteReqBody))
	deleteReq.AddCookie(&http.Cookie{
		Name:  "_turkeyauthcookie",
		Value: vu._turkeyauthcookie,
	})
	if err != nil {
		panic("deleteReq error: " + err.Error())
	}
	resp, err := _httpClient.Do(deleteReq)
	if err != nil && resp.StatusCode != http.StatusOK {
		fmt.Printf("\nerr: %v, resp: %v", err, resp)
		panic("failed @ delete")
	}
	fmt.Printf("\n[%v] deleted", vu.HubId)
}

func (vu *Vuser) Load(ttl time.Duration) {
	fmt.Printf("\n[%v] loading (just time.Sleep for now...)", vu.Id)
	wait := 1 * time.Minute
	for ttl > 0 {
		time.Sleep(wait)
		fmt.Printf("\n[%v] running, ttl: %v, url: %v", vu.Id, ttl, vu.Url)
		ttl -= wait
	}
	fmt.Printf("\n[%v] done", vu.Id)
}

func (vu *Vuser) ToString() string {
	return fmt.Sprintf(`\n{ vu.Id: %v, vu.HubId: %v, vu.tCreate: %v, vu.tReady: %v}`, vu.Id, vu.HubId, vu.TCreate, vu.TReady)
}