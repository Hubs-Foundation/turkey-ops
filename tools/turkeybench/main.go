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
	_turkeyauthcookie = "fterFx-yCNququYpFMZM3pFLb8SKN_hZ22LY31bcsbQ=|1657260837|gtan@mozilla.com"
	useremail         = "gtan@mozilla.com"
	stepWait          = 1 * time.Millisecond
)

//	super lame manual cleanup ...
//	kubectl get ns | grep turkeybench | awk '{print $1}' | xargs kubectl delete ns
//	SELECT CONCAT('DROP DATABASE ', datname,';') FROM pg_database WHERE datname LIKE 'ret_turkeybench%' AND datistemplate=false
var vuBag []*internal.Vuser

func main() {

	reader := bufio.NewReader(os.Stdin)
	fmt.Printf(">> domain: %v, stepWait: %v. type a number to start: ", turkeyDomain, stepWait)
	cmd, _ := reader.ReadString('\n')
	if num, err := strconv.Atoi(dropNewLineChars(cmd)); err == nil {
		addUsers(num)
	} else {
		fmt.Println("bad input: <" + cmd + ">")
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
		fmt.Println("\n---")
		fmt.Println(`>> "load" or "delete" to run vu.load or vu.delete: `)
		fmt.Println(`>> "exit" to quit: `)
		cmd, _ = reader.ReadString('\n')
		if dropNewLineChars(cmd) == "load" {
			var wg_load sync.WaitGroup
			for _, vu := range vuBag {
				wg_load.Add(1)
				go func(vu *internal.Vuser) {
					defer wg_load.Done()
					vu.Load(10 * time.Minute)
				}(vu)
			}
			wg_load.Wait()
		} else if dropNewLineChars(cmd) == "delete" {
			for _, vu := range vuBag {
				vu.Delete()
			}
		} else if dropNewLineChars(cmd) == "exit" {
			break
		} else {
			fmt.Println(">> bad input: <" + cmd + ">")
		}
	}

}

func addUsers(num int) {
	for i := 0; i < num; i++ {
		vuBag = append(vuBag, internal.NewVuser(strconv.Itoa(i),
			turkeyDomain, _turkeyauthcookie, useremail,
			"turkeybench"+strings.ReplaceAll(uuid.New().String(), "-", ""),
		))
	}
}

func dropNewLineChars(str string) string {
	str = strings.ReplaceAll(str, "\r\n", "")
	str = strings.ReplaceAll(str, "\n", "")
	return str
}
