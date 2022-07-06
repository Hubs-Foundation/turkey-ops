package main

import (
	"bufio"
	"fmt"
	"os"
	"strconv"
	"strings"
	"sync"
	"time"

	"main/internal"

	"github.com/google/uuid"
)

//naFfSCreQagnbhL9pgcDC2ZfxY2IzrXLSrsEXMsYUp0=|1656465179|gtan@mozilla.com
var (
	turkeyDomain      = "gtan.myhubs.net"
	_turkeyauthcookie = "KgZZSYKl-LB3jRBSvS1QK5qJVpJmsbZ1KDDFguxl3f4=|1657155037|gtan@mozilla.com"
	useremail         = "gtan@mozilla.com"
	stepWait          = 1 * time.Millisecond
)

//	super lame manual cleanup ...
//	kubectl get ns | grep turkeybench | awk '{print $1}' | xargs kubectl delete ns
//	SELECT CONCAT('DROP DATABASE ', datname,';') FROM pg_database WHERE datname LIKE 'ret_turkeybench%' AND datistemplate=false

func main() {
	vuBag := []*internal.Vuser{}
	for i := 0; i < 10; i++ {
		vuBag = append(vuBag, internal.NewVuser(strconv.Itoa(i),
			turkeyDomain, _turkeyauthcookie, useremail,
			"turkeybench"+strings.ReplaceAll(uuid.New().String(), "-", ""),
		))
	}

	reader := bufio.NewReader(os.Stdin)
	fmt.Printf(">> domain: %v, user count: %v, stepWait: %v. type \"go\" to start: ", turkeyDomain, len(vuBag), stepWait)
	cmd, _ := reader.ReadString('\n')
	if !chkCmd(cmd, "go") {
		return
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

	fmt.Println("\n=========================================================================")
	for _, vu := range vuBag {
		fmt.Println(vu.ToString())
	}

	for {
		fmt.Println(`>> "load" or "delete" to run vu.load or vu.delete: `)
		fmt.Println(`>> "exit" to quit: `)
		cmd, _ = reader.ReadString('\n')
		if chkCmd(cmd, "load") {
			var wg_load sync.WaitGroup
			for _, vu := range vuBag {
				wg_load.Add(1)
				go func(vu *internal.Vuser) {
					defer wg_load.Done()
					vu.Load(10 * time.Minute)
				}(vu)
			}
			wg_load.Wait()
		} else if chkCmd(cmd, "delete") {
			for _, vu := range vuBag {
				vu.Delete()
			}
		} else if chkCmd(cmd, "exit") {
			break
		} else {
			fmt.Println(">> bad input: <" + cmd + ">")
		}
	}

}

func chkCmd(cmd, chk string) bool {
	if cmd == chk+"\n" || cmd == chk+"\r\n" {
		return true
	}
	return false
}
