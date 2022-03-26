package internal

import (
	"net/http"
	"net/http/httputil"
	"net/url"
	"time"
)

type proxyman struct {
	Pool map[string]*httputil.ReverseProxy
}

var Proxyman *proxyman

func (p *proxyman) Init() {
	p.Pool = make(map[string]*httputil.ReverseProxy)
}

func (p *proxyman) Get(target string) *httputil.ReverseProxy {
	if proxy, ok := p.Pool[target]; ok {
		return proxy
	}
	newProxy, err := p.new(target)
	if err != nil {
		Logger.Sugar().Errorf("failed to create new proxy: %v", err)
		return nil
	}
	return newProxy
}

func (p *proxyman) new(target string) (*httputil.ReverseProxy, error) {
	targetUrl, err := url.Parse(target)
	if err != nil {
		return nil, err
	}
	proxy := httputil.NewSingleHostReverseProxy(targetUrl)
	proxy.Transport = &http.Transport{ResponseHeaderTimeout: 1 * time.Minute}

	original := proxy.Director
	proxy.Director = func(r *http.Request) {
		original(r)
		modifyRequest(r, map[string]string{
			"AuthnProxied": "1",
		})
	}
	return proxy, nil

}
