package internal

import (
	"sync"
	"time"
)

//concurrent cached/map for hubs cloud namespace bookkeeping
type HcNsTable struct {
	hcNsTable                map[string]HcNsNotes
	hcSubdomainNsLookupTable map[string]string
}

//singleton instance
var HC_NS_TABLE = &HcNsTable{
	hcNsTable:                make(map[string]HcNsNotes),
	hcSubdomainNsLookupTable: make(map[string]string),
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

func (t HcNsTable) Set(nsName string, notes HcNsNotes) {
	mu.Lock()
	defer mu.Unlock()

	t.hcNsTable[nsName] = notes
	t.hcSubdomainNsLookupTable[notes.Labels["subdomain"]] = nsName
}

func (t HcNsTable) Del(nsName string) {
	mu.Lock()
	defer mu.Unlock()

	delete(t.hcNsTable, nsName)
}

// func (t HcNsTable) HasSubdomain(subdomain string) bool {
// 	mu.Lock()
// 	defer mu.Unlock()

// 	_, has := t.hcSubdomainNsLookupTable[subdomain]
// 	return has
// }

func (t HcNsTable) GetNsName(subdomain string) string {
	mu.Lock()
	defer mu.Unlock()

	return t.hcSubdomainNsLookupTable[subdomain]
}
