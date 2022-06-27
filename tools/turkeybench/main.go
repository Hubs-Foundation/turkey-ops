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
	_turkeyauthcookie = "19-qebCRg7kl0-_xCKy2gbkcN1v89n8ABvQlDZ-TojM=|1656377422|gtan@mozilla.com"
	useremail         = "gtan@mozilla.com"
	stepWait          = 250 * time.Millisecond
)

// kubectl get ns | grep turkeybench | awk '{print $1}' | xargs kubectl delete ns
func main() {
	vuBag := []*internal.Vuser{}
	for i := 0; i < 25; i++ {
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

	fmt.Println("=========================================================================")
	for _, vu := range vuBag {
		fmt.Println(vu.ToString())
	}

}
