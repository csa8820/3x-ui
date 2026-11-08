package website

import (
	"html/template"
	"net/http"
	"net/http/httputil"
	"net/url"
	"strings"
)

var page = template.Must(template.New("home").Parse(`<!doctype html>
<html lang="en"><head><meta charset="utf-8"><meta name="viewport" content="width=device-width,initial-scale=1">
<meta name="description" content="{{.Description}}"><title>{{.Title}}</title><style>
*{box-sizing:border-box}body{margin:0;background:#f6f5f1;color:#20352e;font:17px/1.7 system-ui,sans-serif}
header,main,footer{max-width:1100px;margin:auto;padding:30px 7%}header{border-bottom:1px solid #d8dfd5;font-weight:650}
main{padding-top:100px;padding-bottom:100px}h1{font-size:clamp(38px,7vw,76px);line-height:1.12;letter-spacing:-.05em;max-width:850px}
.intro{max-width:680px;white-space:pre-line;color:#53655b}.grid{display:grid;grid-template-columns:repeat(auto-fit,minmax(180px,1fr));gap:24px;margin-top:65px}
article{border-top:2px solid #789484;padding-top:18px}article p,footer{color:#65736b;font-size:14px}footer{border-top:1px solid #d8dfd5}
</style></head><body><header>{{.Title}}</header><main><p>IDEAS · CRAFT · CONNECTION</p>
<h1>{{.Title}}</h1><p class="intro">{{.Description}}</p><section class="grid">
<article><h2>Thoughtful work</h2><p>We turn clear ideas into useful, considered experiences.</p></article>
<article><h2>Made with care</h2><p>Good details make a lasting difference.</p></article>
<article><h2>Always growing</h2><p>We keep learning, exploring and improving our work.</p></article>
</section></main><footer>© {{.Title}}. All rights reserved.</footer></body></html>`))

func NewHandler(config Config, proxyPath, upstream string, transport http.RoundTripper) http.Handler {
	if config.Title == "" {
		config.Title = "A place for better ideas"
	}
	if config.Description == "" {
		config.Description = "An independent space for thoughtful projects, useful ideas and new possibilities."
	}
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
		if strings.HasPrefix(r.URL.Path, proxyPath) {
			_ = http.NewResponseController(w).EnableFullDuplex()
			proxy.ServeHTTP(w, r)
			return
		}
		w.Header().Set("X-Content-Type-Options", "nosniff")
		if r.URL.Path != "/" {
			http.NotFound(w, r)
			return
		}
		if r.Method != http.MethodGet && r.Method != http.MethodHead {
			w.Header().Set("Allow", "GET, HEAD")
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
			return
		}
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		if r.Method == http.MethodGet {
			_ = page.Execute(w, config)
		}
	})
}
