package internal

import (
	"container/list"
	"fmt"
	"sync"
	"time"
)

type k8Man struct {
	_busy      bool
	mu         sync.Mutex
	worklog    *list.List
	mu_worklog sync.Mutex
}

type k8WorklogEntry struct {
	work  string
	event string
	at    time.Time
}

func New_k8Man() *k8Man {

	_worklog := list.New()
	_worklog.PushBack(
		k8WorklogEntry{work: "", event: "init", at: time.Now()},
	)
	return &k8Man{
		_busy:   false,
		worklog: _worklog,
	}
}
func (k *k8Man) IsBusy() bool {
	k.mu.Lock()
	defer k.mu.Unlock()
	return k._busy
}

func (k *k8Man) WriteWorkLog(entry k8WorklogEntry) {
	k.mu_worklog.Lock()
	defer k.mu_worklog.Unlock()
	k.worklog.PushBack(entry)
	if k.worklog.Len() > 100 {
		k.worklog.Remove(k.worklog.Front())
	}
}

func (k *k8Man) DumpWorkLog() string {
	k.mu_worklog.Lock()
	defer k.mu_worklog.Unlock()

	dump := ""
	ele := k.worklog.Front()
	for ele != nil {
		entry := ele.Value.(k8WorklogEntry)
		dump += fmt.Sprintf("\n  [%v]***%v***at %v", entry.event, entry.work, entry.at.Format(time.RFC822))
		ele = ele.Next()
	}

	return dump
}
func (k *k8Man) WorkBegin(work string) {
	k.wantToStart(work)
	k.mu.Lock()
	defer k.mu.Unlock()
	k._busy = true
	k.WriteWorkLog(
		k8WorklogEntry{work: work, event: "WorkBegin", at: time.Now()},
	)
}

func (k *k8Man) WorkEnd(work string) {
	k.mu.Lock()
	defer k.mu.Unlock()
	k._busy = false
	k.WriteWorkLog(
		k8WorklogEntry{work: work, event: "WorkEnd", at: time.Now()},
	)
}

func (k *k8Man) wantToStart(work string) {
	if !k.IsBusy() {
		return
	}
	k.WriteWorkLog(
		k8WorklogEntry{work: work, event: "WantToStart", at: time.Now()},
	)
	for k.IsBusy() {
		time.Sleep(1 * time.Second)
	}
}
