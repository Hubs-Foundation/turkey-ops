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
var mu sync.Mutex

func InitProxyman() {
	Proxyman = &proxyman{
		Pool: make(map[string]*httputil.ReverseProxy),
	}

}

func (p *proxyman) Get(target string) (*httputil.ReverseProxy, error) {
	mu.Lock()
	defer mu.Unlock()

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
		TLSClientConfig: &tls.Config{InsecureSkipVerify: true},
		// Proxy: http.ProxyFromEnvironment,
		// DialContext: (&net.Dialer{
		// 	Timeout:   900 * time.Second,
		// 	KeepAlive: 900 * time.Second,
		// }).DialContext,
		MaxIdleConns:          100,
		IdleConnTimeout:       900 * time.Second,
		TLSHandshakeTimeout:   10 * time.Second,
		ExpectContinueTimeout: 1 * time.Second,
	}

	return proxy, nil

}
