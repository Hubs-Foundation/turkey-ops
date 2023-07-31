package internal

import (
	"encoding/json"
	"os"
	"sync"
	"time"

	"github.com/tanfarming/goutils/pkg/filelocker"
)

type trcCache struct {
	File       string
	Book       map[string]TrcCacheData
	Updated_at time.Time
	mu         sync.RWMutex
}

type TrcCacheData struct {
	HubId        string    `json:"hub_id"`
	OwnerEmail   string    `json:"owner_email"`
	IsRunning    bool      `json:"is_running"`
	Collected_at time.Time `json:"collected_at"`
}

func NewTrcCache(File string) *trcCache {

	b := &trcCache{
		File: File,
		Book: map[string]TrcCacheData{},
	}

	if _, err := os.Stat(b.File); err != nil {
		os.WriteFile(b.File, []byte{}, 0600)
	}

	return b

}

func (b *trcCache) Get(subdomain string) TrcCacheData {
	b.mu.RLock()
	defer b.mu.RUnlock()
	return b.Book[subdomain]
}

func (b *trcCache) Set(subdomain string, data TrcCacheData) {

	b.mu.Lock()
	defer b.mu.Unlock()

	b.Book[subdomain] = data

}

// set to file
func (b *trcCache) UpdateFile() error {
	f, err := os.OpenFile(b.File, os.O_WRONLY, 0600)
	if err != nil {
		return err
	}
	defer f.Close()

	if err = filelocker.Lock(f); err != nil {
		return err
	}

	//sync up
	fBytes, err := os.ReadFile(b.File)
	if err != nil {
		return err
	}
	m := map[string]TrcCacheData{}
	if len(fBytes) == 0 {
		return nil
	}
	err = json.Unmarshal(fBytes, &m)
	if err != nil {
		return err
	}
	for k, v := range m {
		b.Book[k] = v
	}

	//write file
	bytes, err := json.Marshal(b.Book)
	if err != nil {
		return err
	}
	_, err = f.Write(bytes)

	if err := filelocker.Unlock(f); err != nil {
		return err
	}

	return err
}

func (b *trcCache) getFromFile() (map[string]TrcCacheData, error) {

	f, err := os.OpenFile(b.File, os.O_RDONLY, 0600)
	if err != nil {
		return nil, err
	}
	defer f.Close()

	if err = filelocker.RLock(f); err != nil {
		return nil, err
	}

	fBytes, err := os.ReadFile(b.File)
	if err != nil {
		return nil, err
	}
	if err := filelocker.Unlock(f); err != nil {
		return nil, err
	}

	m := map[string]TrcCacheData{}
	if len(fBytes) == 0 {
		return m, nil
	}

	err = json.Unmarshal(fBytes, &m)
	if err != nil {
		return m, err
	}

	return m, err
}
