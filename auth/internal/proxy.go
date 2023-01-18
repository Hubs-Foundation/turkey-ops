package internal

import (
	"crypto/tls"
	"net/http"
	"net/http/httputil"
	"net/url"
	"sync"
	"time"
)

type proxyman struct {
	Pool map[string]*httputil.ReverseProxy
}

var Proxyman *proxyman
var mu_proxy sync.Mutex

func InitProxyman() {
	Proxyman = &proxyman{
		Pool: make(map[string]*httputil.ReverseProxy),
	}

}

func (p *proxyman) Get(target string) (*httputil.ReverseProxy, error) {
	mu_proxy.Lock()
	defer mu_proxy.Unlock()

	if proxy, ok := p.Pool[target]; ok {
		return proxy, nil
	}
	Logger.Debug("making new proxy for: " + target)
	newProxy, err := p.new(target)
	if err != nil {
		Logger.Sugar().Errorf("failed to create new proxy: %v", err)
		return nil, err
	}
	p.Pool[target] = newProxy
	return newProxy, nil
}

func (p *proxyman) new(target string) (*httputil.ReverseProxy, error) {
	targetUrl, err := url.Parse(target)
	if err != nil {
		return nil, err
	}
	proxy := httputil.NewSingleHostReverseProxy(targetUrl)
	proxy.Transport = &http.Transport{
		TLSClientConfig:       &tls.Config{InsecureSkipVerify: true},
		MaxIdleConns:          100,
		IdleConnTimeout:       1000 * time.Second,
		TLSHandshakeTimeout:   10 * time.Second,
		ExpectContinueTimeout: 2 * time.Second,
	}

	return proxy, nil

}
