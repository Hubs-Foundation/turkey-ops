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
	useremail = "gtan@mozilla.com"
	stepWait  = 1 * time.Millisecond
)

//	super lame manual cleanup ...
//	kubectl get ns | grep turkeybench | awk '{print $1}' | xargs kubectl delete ns
//	SELECT CONCAT('DROP DATABASE ', datname,';') FROM pg_database WHERE datname LIKE 'ret_turkeybench%' AND datistemplate=false
var vuBag []*internal.Vuser

func main() {

	// userCnt := *flag.Int("u", 10, "number of virtual users, int")
	// turkeyDomain := *flag.String("d", "error: not-provided", "ie. \"dev.myhubs.net\", string")
	// token := *flag.String("t", "error: not-provided", "value of _turkeyauthtoken, string")
	// flag.Parse()
	userCnt, _ := strconv.Atoi(os.Getenv("userCnt"))
	turkeyDomain := os.Getenv("domain")
	token := os.Getenv("token")
	fmt.Printf(">> \ndomain: %v, \nstepWait: %v, \nuserCnt: %v, \ntoken: %v", turkeyDomain, stepWait, userCnt, token)

	addUsers(userCnt, turkeyDomain, token)

	reader := bufio.NewReader(os.Stdin)

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
		cmd, _ := reader.ReadString('\n')
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

func addUsers(num int, turkeyDomain, token string) {
	for i := 0; i < num; i++ {
		vuBag = append(
			vuBag,
			internal.NewVuser(
				strconv.Itoa(i),
				turkeyDomain, token, useremail,
				"turkeybench"+strings.ReplaceAll(uuid.New().String(), "-", ""),
			),
		)
	}
}

func dropNewLineChars(str string) string {
	str = strings.ReplaceAll(str, "\r\n", "")
	str = strings.ReplaceAll(str, "\n", "")
	return str
}
