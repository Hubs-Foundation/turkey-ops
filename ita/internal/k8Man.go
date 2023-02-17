package internal

import (
	"sync"
	"time"
)

type k8Man struct {
	isBusy     bool
	mu         sync.Mutex
	worklog    []k8WorklogEntry
	mu_worklog sync.Mutex
}

type k8WorklogEntry struct {
	work  string
	event string
	at    time.Time
}

func New_k8Man() *k8Man {
	return &k8Man{
		isBusy: false,
		worklog: []k8WorklogEntry{
			{work: "init", event: "", at: time.Now()},
		},
	}
}
func (k *k8Man) IsBusy() bool {
	k.mu.Lock()
	defer k.mu.Unlock()
	return k.isBusy
}

func (k *k8Man) WriteWorkLog(entry k8WorklogEntry) {
	k.mu_worklog.Lock()
	defer k.mu_worklog.Unlock()
	k.worklog = append(k.worklog, entry)
}

func (k *k8Man) DumpWorkLog() []k8WorklogEntry {
	k.mu_worklog.Lock()
	defer k.mu_worklog.Unlock()
	return k.worklog
}
func (k *k8Man) WorkBegin(work string) {
	k.mu.Lock()
	defer k.mu.Unlock()
	k.isBusy = true
	k.WriteWorkLog(
		k8WorklogEntry{work: work, event: "WorkBegin", at: time.Now()},
	)
}

func (k *k8Man) WorkEnd(work string) {
	k.mu.Lock()
	defer k.mu.Unlock()
	k.isBusy = false
	k.WriteWorkLog(
		k8WorklogEntry{work: work, event: "WorkEnd", at: time.Now()},
	)
}

func (k *k8Man) WantsToStart(work string) {
	if !k.IsBusy() {
		return
	}
	k.WriteWorkLog(
		k8WorklogEntry{work: work, event: "WantsToStart", at: time.Now()},
	)
	for k.IsBusy() {
		time.Sleep(1 * time.Second)
	}
}
