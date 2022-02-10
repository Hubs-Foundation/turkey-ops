package internal

import (
	"sync"
	"time"
)

//concurrent cached/map for hubs cloud namespace bookkeeping
type HcNsTable struct{}

//singleton instance
var HC_NS_TABLE = &HcNsTable{}

var mu sync.Mutex

type HcNsNotes struct {
	Lastchecked time.Time
	Labels      map[string]string
}

var hcNsTable = map[string]HcNsNotes{}

func (t HcNsTable) Get(nsName string) HcNsNotes {
	mu.Lock()
	defer mu.Unlock()

	return hcNsTable[nsName]
}

func (t HcNsTable) Set(nsName string, value HcNsNotes) {
	mu.Lock()
	defer mu.Unlock()

	hcNsTable[nsName] = value
}

func (t HcNsTable) Del(nsName string) {
	mu.Lock()
	defer mu.Unlock()

	delete(hcNsTable, nsName)
}

func (t HcNsTable) Has(nsName string) bool {
	mu.Lock()
	defer mu.Unlock()

	_, has := hcNsTable[nsName]
	return has
}
