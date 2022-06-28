package main

import (
	"fmt"
	"strconv"
	"strings"
	"sync"
	"time"

	"main/internal"

	"github.com/google/uuid"
)

var (
	turkeyDomain      = "gtan.myhubs.net"
	_turkeyauthcookie = "RahyHXcnssof4VbDOwei8I6iVSDOKX90I9vF5Liy6E0=|1656424001|gtan@mozilla.com"
	useremail         = "gtan@mozilla.com"
	stepWait          = 1 * time.Millisecond
)

//	super lame manual cleanup ...
//	kubectl get ns | grep turkeybench | awk '{print $1}' | xargs kubectl delete ns
//	SELECT CONCAT('DROP DATABASE ', datname,';') FROM pg_database WHERE datname LIKE 'ret_turkeybench%' AND datistemplate=false

func main() {
	vuBag := []*internal.Vuser{}
	for i := 0; i < 100; i++ {
		vuBag = append(vuBag, internal.NewVuser(strconv.Itoa(i),
			turkeyDomain, _turkeyauthcookie, useremail,
			"turkeybench"+strings.ReplaceAll(uuid.New().String(), "-", ""),
		))
	}
	var wg_create sync.WaitGroup
	for _, vu := range vuBag {
		wg_create.Add(1)
		go func(vu *internal.Vuser) {
			defer wg_create.Done()
			vu.Create()
		}(vu)
		time.Sleep(stepWait)
	}
	wg_create.Wait()

	var wg_load sync.WaitGroup
	for _, vu := range vuBag {
		wg_load.Add(1)
		go func(vu *internal.Vuser) {
			defer wg_load.Done()
			vu.Load(5 * time.Minute)
		}(vu)
	}
	wg_load.Wait()

	fmt.Println("\n=========================================================================")
	for _, vu := range vuBag {
		fmt.Println(vu.ToString())
	}

}
