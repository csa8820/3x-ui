package website

import (
	"net/http"
	"net/http/httputil"
	"net/url"
	"strings"
)

func NewHandler(config Config, proxyPath, upstream string, transport http.RoundTripper) http.Handler {
	pageHandler := TemplateHandler(config)
	target, _ := url.Parse(upstream)
	proxy := &httputil.ReverseProxy{
		Rewrite: func(r *httputil.ProxyRequest) {
			r.SetURL(target)
			r.Out.Host = r.In.Host
			r.SetXForwarded()
			r.Out.Header.Set("X-3x-UI-Website", "1")
		},
		Transport: transport, FlushInterval: -1,
		ErrorHandler: func(w http.ResponseWriter, _ *http.Request, _ error) {
			http.Error(w, "Service unavailable", http.StatusServiceUnavailable)
		},
	}
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if proxyPath != "" && strings.HasPrefix(r.URL.Path, proxyPath) {
			_ = http.NewResponseController(w).EnableFullDuplex()
			proxy.ServeHTTP(w, r)
			return
		}
		pageHandler.ServeHTTP(w, r)
	})
}
