package main

import (
	"fmt"
	"strings"

	"main/internal"

	"github.com/google/uuid"
)

func main() {

	vu := &internal.Vuser{
		HubId: "tb" + strings.ReplaceAll(uuid.New().String(), "-", ""),
	}
	vu.Create()
	vu.Load()
	vu.Delete()
	fmt.Printf("\n###### %v", vu.ToString())
}
