package internal

import (
	"sync"
	"time"
)

//concurrent cached/map for hubs cloud namespace bookkeeping
type HcNsTable struct {
	hcNsTable map[string]HcNsNotes
}

//singleton instance
var HC_NS_TABLE = &HcNsTable{
	hcNsTable: make(map[string]HcNsNotes),
}

var mu sync.Mutex

type HcNsNotes struct {
	Lastchecked time.Time
	Labels      map[string]string
}

func (t HcNsTable) Get(nsName string) HcNsNotes {
	mu.Lock()
	defer mu.Unlock()

	return t.hcNsTable[nsName]
}

func (t HcNsTable) Set(nsName string, value HcNsNotes) {
	mu.Lock()
	defer mu.Unlock()

	t.hcNsTable[nsName] = value
}

func (t HcNsTable) Del(nsName string) {
	mu.Lock()
	defer mu.Unlock()

	delete(t.hcNsTable, nsName)
}

func (t HcNsTable) Has(nsName string) bool {
	mu.Lock()
	defer mu.Unlock()

	_, has := t.hcNsTable[nsName]
	return has
}
