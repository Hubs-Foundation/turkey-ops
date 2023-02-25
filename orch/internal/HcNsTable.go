package internal

import (
	"sync"
	"time"
)

//concurrent cached/map for hubs cloud namespace bookkeeping
type HcNsMan struct {
	mu sync.Mutex
	// nsName : HcNsNotes
	hcNsTable map[string]HcNsNotes
	// subdomain : nsName
	hcSubdomainNsLookupTable map[string]string
}

//singleton instance
var HC_NS_MAN = &HcNsMan{
	hcNsTable:                make(map[string]HcNsNotes),
	hcSubdomainNsLookupTable: make(map[string]string),
}

type HcNsNotes struct {
	Lastchecked time.Time
	Labels      map[string]string
}

func (t *HcNsMan) Get(nsName string) HcNsNotes {
	t.mu.Lock()
	defer t.mu.Unlock()

	return t.hcNsTable[nsName]
}

func (t *HcNsMan) Set(nsName string, notes HcNsNotes) {
	t.mu.Lock()
	defer t.mu.Unlock()

	t.hcNsTable[nsName] = notes
	t.hcSubdomainNsLookupTable[notes.Labels["subdomain"]] = nsName
}

func (t *HcNsMan) Del(nsName string) {
	t.mu.Lock()
	defer t.mu.Unlock()

	delete(t.hcNsTable, nsName)
}

func (t *HcNsMan) Dump() map[string]HcNsNotes {
	t.mu.Lock()
	defer t.mu.Unlock()

	return t.hcNsTable
}

// func (t HcNsMan) HasSubdomain(subdomain string) bool {
// 	mu.Lock()
// 	defer mu.Unlock()

// 	_, has := t.hcSubdomainNsLookupTable[subdomain]
// 	return has
// }

func (t *HcNsMan) GetNsName(subdomain string) string {
	t.mu.Lock()
	defer t.mu.Unlock()

	return t.hcSubdomainNsLookupTable[subdomain]
}
