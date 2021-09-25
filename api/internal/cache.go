package internal

import (
	"fmt"
	"sync"
	"time"
)

type CacheBox struct {
	sessBox *SessBox
	metaBox *MetaBox
}
type SessBox struct {
	sync.RWMutex
	internal map[string]*CacheBoxSessData
}
type CacheBoxSessData struct {
	// UserData *UserData
	UserData map[string]string
	SseChan  chan string
}

func NewSessBox() *SessBox {
	return &SessBox{internal: make(map[string]*CacheBoxSessData)}
}
func (sb *SessBox) Load(key string) *CacheBoxSessData {
	sb.RLock()
	r, _ := sb.internal[key]
	sb.RUnlock()
	return r
}
func (sb *SessBox) Store(key string, value *CacheBoxSessData) {
	sb.Lock()
	sb.internal[key] = value
	sb.Unlock()
}
func (sb *SessBox) Delete(key string) {
	sb.Lock()
	delete(sb.internal, key)
	sb.Unlock()
}

type MetaBox struct {
	sync.RWMutex
	internal map[string]*CacheBoxMetaData
}
type CacheBoxMetaData struct {
	Lifespan time.Duration
	TimeExp  time.Time
}

func NewMetaBox() *MetaBox {
	return &MetaBox{internal: make(map[string]*CacheBoxMetaData)}
}
func (mb *MetaBox) Load(key string) *CacheBoxMetaData {
	mb.RLock()
	r, _ := mb.internal[key]
	mb.RUnlock()
	return r
}
func (mb *MetaBox) Store(key string, value *CacheBoxMetaData) {
	mb.Lock()
	mb.internal[key] = value
	mb.Unlock()
}
func (mb *MetaBox) Delete(key string) {
	mb.Lock()
	delete(mb.internal, key)
	mb.Unlock()
}

func (sess *CacheBoxSessData) PushMsg(msg string) {

	go func() {
		if sess == nil {
			Logger.Println("WARNING @ PushMsg: no session. \n message dropped = <" + msg + ">")
			return
		}
		attempt := 0
		delay := time.Millisecond * 1
		delayStep := 500 * time.Millisecond
		maxAttempt := 10
		for sess.SseChan == nil {
			time.Sleep(delayStep)
			delay = time.Second * 1
			if attempt >= maxAttempt {
				Logger.Println("WARNING @ PushMsg: no channel. \n message dropped = <" + msg + ">")
				return
			}
			attempt = attempt + 1
		}
		time.Sleep(delay)
		sess.SseChan <- fmt.Sprintf(msg)
		Logger.Println("PushMsg: <" + msg + ">")
	}()
	time.Sleep(5e7)
}

func NewCacheBox() CacheBox {
	cb := CacheBox{
		sessBox: NewSessBox(),
		metaBox: NewMetaBox(),
	}
	return cb
}

func (cb *CacheBox) Put(key string, value *CacheBoxSessData, Lifespan time.Duration) {
	cb.sessBox.Store(key, value)
	cb.metaBox.Store(key, &CacheBoxMetaData{
		Lifespan: Lifespan,
		TimeExp:  time.Now().UTC().Add(Lifespan),
	})
}

func (cb *CacheBox) PUT(key string, value *CacheBoxSessData) {
	cb.sessBox.Store(key, value)
}

func (cb *CacheBox) Load(key string) *CacheBoxSessData {
	v := cb.sessBox.Load(key)
	if v != nil {
		if cb.metaBox.Load(key) != nil && time.Now().UTC().After(cb.metaBox.Load(key).TimeExp) {
			cb.Delete(key)
			return nil
		}
		cb.Renew(key)
		return v
	}

	return nil
}

func (cb *CacheBox) Delete(key string) {

	cb.sessBox.Delete(key)
	cb.metaBox.Delete(key)
}

func (cb *CacheBox) Renew(key string) {
	Lifespan := cb.metaBox.Load(key).Lifespan
	cb.metaBox.Store(key, &CacheBoxMetaData{
		Lifespan: Lifespan,
		TimeExp:  time.Now().UTC().Add(Lifespan),
	})
}
