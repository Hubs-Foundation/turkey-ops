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
	turkeyDomain     string //	ie. "gtan.myhubs.net"
	_turkeyauthtoken string // ie. "WmeluLWXgwZ_U3Vk6k073-dtT9jNCd1WJ6_7vRkhKh4=|1655339779|gtan@mozilla.com"
	useremail        string // 	ie. "gtan@mozilla.com"

	Id      string
	HubId   string
	TCreate time.Duration
	TReady  time.Duration
	Url     string
}

func NewVuser(Id, domain, authcookie, email, hubId string) *Vuser {
	return &Vuser{
		turkeyDomain:     domain,
		_turkeyauthtoken: authcookie,
		useremail:        email,
		HubId:            hubId,
		Url:              "https://" + hubId + "." + domain + "/?skipadmin",
		Id:               Id,
	}
}

func (vu *Vuser) Create() {
	//create
	createReqBody, _ := json.Marshal(map[string]string{
		"hub_id":    vu.HubId,
		"useremail": vu.useremail,
		"tier":      "mvp2",
	})
	createReq, err := http.NewRequest("POST", "https://orch."+vu.turkeyDomain+"/hc_instance", bytes.NewBuffer(createReqBody))
	createReq.AddCookie(&http.Cookie{
		Name:  "_turkeyauthtoken",
		Value: vu._turkeyauthtoken,
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
	fmt.Printf("\n%v vu.Id[%v] -- [tCreate: %v]", time.Now().Format("15:04:05"), vu.Id, vu.TCreate)
	//wait for ret
	tStart = time.Now()
	timeout := time.Now().Add(30 * time.Minute)
	retReq, _ := http.NewRequest("GET", "https://"+vu.HubId+"."+vu.turkeyDomain+"/?skipadmin", nil)
	// fmt.Printf("\nreq: %v", retReq.URL)
	resp, err = _httpClient.Do(retReq)
	for err != nil || resp.StatusCode != http.StatusOK {
		time.Sleep(15 * time.Second)
		ttl := time.Until(timeout)
		if resp != nil {
			// bodyBytes, _ := io.ReadAll(resp.Body)
			// fmt.Printf("\n---[waiting-for-ret] got: %v[%v], retrying, ttl: %v, hubId: %v", resp.StatusCode, string(bodyBytes), ttl, vu.HubId)
			fmt.Printf("\n%v---[waiting-for-ret] [vu.id:%v] received: %v, retrying, ttl: %v, hubId: %v", time.Now().Format("15:04:05"), vu.Id, resp.StatusCode, ttl, vu.HubId)
		} else {
			fmt.Printf("\n%v---[waiting-for-ret] [vu.id:%v] received: %v, retrying, ttl: %v, hubId: %v", time.Now().Format("15:04:05"), vu.Id, err, ttl, vu.HubId)
		}
		if ttl < 0 {
			fmt.Printf("\n%v---[waiting-for-ret] [vu.id:%v] err: timeout -- hubId %v", time.Now().Format("15:04:05"), vu.Id, vu.HubId)
			vu.TReady = -1
			break
		}
		resp, err = _httpClient.Do(retReq)
	}
	if vu.TReady != -1 {
		vu.TReady = time.Since(tStart)
	}
	fmt.Printf("\n %v[%v] [tReady: %v] @ %v", time.Now().Format("15:04:05"), vu.Id, vu.TReady, vu.Url)
}

func (vu *Vuser) Delete() {
	fmt.Printf("\n[deleting: %v]", vu.HubId)
	//delete
	deleteReqBody, _ := json.Marshal(map[string]string{
		"hub_id": vu.HubId,
	})
	deleteReq, err := http.NewRequest("DELETE", "https://orch."+vu.turkeyDomain+"/hc_instance", bytes.NewBuffer(deleteReqBody))
	deleteReq.AddCookie(&http.Cookie{
		Name:  "_turkeyauthtoken",
		Value: vu._turkeyauthtoken,
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
	time.Sleep(ttl)
	return
	// fmt.Printf("\n[%v] loading (just time.Sleep for now...)", vu.Id)
	// wait := 1 * time.Minute
	// for ttl > 0 {
	// 	time.Sleep(wait)
	// 	fmt.Printf("\n[%v] running, ttl: %v, url: %v", vu.Id, ttl, vu.Url)
	// 	ttl -= wait
	// }
	// fmt.Printf("\n[%v] done", vu.Id)
}

func (vu *Vuser) ToString() string {
	return fmt.Sprintf(
		`{ vu.Id: %v, vu.tCreate: %v, vu.tReady: %v, vu.Url: %v}`, vu.Id, vu.TCreate, vu.TReady, vu.Url)
}
