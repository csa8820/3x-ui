package website

import (
	"bytes"
	"html/template"
	"net/http"
	"os"
	"path/filepath"
	"strings"

	"github.com/mhsanaei/3x-ui/v3/internal/config"
)

type TemplateInfo struct {
	ID         string `json:"id"`
	Name       string `json:"name"`
	Category   string `json:"category"`
	Source     string `json:"source"`
	License    string `json:"license"`
	Repository string `json:"repository,omitempty"`
	Installed  bool   `json:"installed"`
}

var builtinTemplates = []TemplateInfo{
	{ID: "studio", Name: "Independent studio", Category: "Studio", Source: "builtin", Installed: true},
	{ID: "journal", Name: "Field notes", Category: "Blog", Source: "builtin", Installed: true},
	{ID: "portfolio", Name: "Selected works", Category: "Portfolio", Source: "builtin", Installed: true},
	{ID: "business", Name: "North & Co.", Category: "Business", Source: "builtin", Installed: true},
	{ID: "docs", Name: "Knowledge garden", Category: "Documentation", Source: "builtin", Installed: true},
	{ID: "minimal", Name: "Quiet space", Category: "Personal", Source: "builtin", Installed: true},
}

var remoteTemplates = []TemplateInfo{
	{ID: "sb-agency", Name: "Agency", Category: "Business", Repository: "startbootstrap-agency"},
	{ID: "sb-clean-blog", Name: "Clean Blog", Category: "Blog", Repository: "startbootstrap-clean-blog"},
	{ID: "sb-freelancer", Name: "Freelancer", Category: "Portfolio", Repository: "startbootstrap-freelancer"},
	{ID: "sb-landing-page", Name: "Landing Page", Category: "Landing page", Repository: "startbootstrap-landing-page"},
	{ID: "sb-creative", Name: "Creative", Category: "Portfolio", Repository: "startbootstrap-creative"},
	{ID: "sb-grayscale", Name: "Grayscale", Category: "Landing page", Repository: "startbootstrap-grayscale"},
	{ID: "sb-resume", Name: "Resume", Category: "Personal", Repository: "startbootstrap-resume"},
	{ID: "sb-business-casual", Name: "Business Casual", Category: "Business", Repository: "startbootstrap-business-casual"},
	{ID: "sb-stylish-portfolio", Name: "Stylish Portfolio", Category: "Portfolio", Repository: "startbootstrap-stylish-portfolio"},
}

func TemplatesDir() string          { return filepath.Join(config.GetDBFolderPath(), "site-templates") }
func TemplateRoot(id string) string { return filepath.Join(TemplatesDir(), id) }

func Catalog() []TemplateInfo {
	result := append([]TemplateInfo{}, builtinTemplates...)
	for _, item := range remoteTemplates {
		item.Source, item.License = "Start Bootstrap", "MIT"
		item.Installed = templateInstalled(item.ID)
		result = append(result, item)
	}
	entries, _ := os.ReadDir(TemplatesDir())
	for _, entry := range entries {
		if entry.IsDir() && strings.HasPrefix(entry.Name(), "custom-") && templateInstalled(entry.Name()) {
			result = append(result, TemplateInfo{ID: entry.Name(), Name: strings.TrimPrefix(entry.Name(), "custom-"), Category: "Imported", Source: "ZIP", Installed: true})
		}
	}
	return result
}

func templateInstalled(id string) bool {
	_, err := os.Stat(filepath.Join(TemplateRoot(id), "index.html"))
	return err == nil
}

func ValidTemplate(id string) bool {
	if id == "" {
		return true
	}
	for _, item := range Catalog() {
		if item.ID == id && item.Installed {
			return true
		}
	}
	return false
}

var variants = map[string]string{
	"journal":   `<p class="eyebrow">FIELD NOTES / JOURNAL</p><h1>{{.Title}}</h1><p>{{.Description}}</p><section><article><small>01 — Ideas</small><h2>Finding a little clarity</h2><p>Notes on everyday life, creative work and things worth remembering.</p></article><article><small>02 — Practice</small><h2>A slower way to build</h2><p>Small steps, useful lessons and a little more intention.</p></article></section>`,
	"portfolio": `<p class="eyebrow">DESIGN & DIRECTION</p><h1>{{.Title}}</h1><p>{{.Description}}</p><section><article class="tile"><span>01</span><h2>Form & function</h2><p>Visual identity · Editorial</p></article><article class="tile second"><span>02</span><h2>Everyday objects</h2><p>Research · Digital design</p></article></section>`,
	"business":  `<p class="eyebrow">INDEPENDENT THINKING. SHARED PROGRESS.</p><h1>{{.Title}}</h1><p>{{.Description}}</p><a href="#services" class="button">Discover our approach ↗</a><section id="services"><article><h2>Strategy</h2><p>Make sense of complex questions and find a clear direction.</p></article><article><h2>Craft</h2><p>Create considered experiences with care and purpose.</p></article><article><h2>Partnership</h2><p>Build lasting work through honest collaboration.</p></article></section>`,
	"docs":      `<p class="eyebrow">A HOME FOR USEFUL KNOWLEDGE</p><h1>{{.Title}}</h1><p>{{.Description}}</p><section><article><h2>Getting started →</h2><p>Find your bearings and discover the essentials.</p></article><article><h2>Guides →</h2><p>Practical notes to help you move forward.</p></article><article><h2>Reference →</h2><p>Details, definitions and further reading.</p></article></section>`,
	"minimal":   `<p class="eyebrow">WELCOME</p><h1>{{.Title}}<span class="dot">.</span></h1><p>{{.Description}}</p><div class="line"></div><p>Creating. Learning. Making room for what comes next.</p>`,
}

func TemplateHandler(c Config) http.Handler {
	if c.Template == "studio" || c.Template == "" {
		return NewHandler(c, "", "http://localhost", http.DefaultTransport)
	}
	if body, ok := variants[c.Template]; ok {
		if c.Title == "" {
			c.Title = "A place for new ideas"
		}
		if c.Description == "" {
			c.Description = "Thoughtful projects, useful notes and an independent point of view."
		}
		t := template.Must(template.New("variant").Parse(`<!doctype html><html lang="en"><head><meta charset="utf-8"><meta name="viewport" content="width=device-width,initial-scale=1"><title>{{.Title}}</title><style>
body{margin:0;background:#f5f1e8;color:#282c27;font:17px/1.7 system-ui}header,main,footer{max-width:1120px;margin:auto;padding:28px 6%}header{border-bottom:1px solid #ccc7bb;font-weight:700}main{padding-top:90px;padding-bottom:100px}h1{font-size:clamp(42px,8vw,90px);line-height:1.05;letter-spacing:-.055em;max-width:900px;margin:26px 0}main>p{max-width:660px;white-space:pre-line}.eyebrow,small{font-size:12px;letter-spacing:.13em}section{display:grid;grid-template-columns:repeat(auto-fit,minmax(230px,1fr));gap:24px;margin-top:70px}article{border-top:1px solid;padding-top:24px}article p{font-size:15px;opacity:.75}.tile{background:#d0d7c4;min-height:230px;padding:32px;border:0}.second{background:#ddc9bd}.button{display:inline-block;margin-top:25px;background:#293c33;color:white;padding:12px 24px;text-decoration:none}.line{height:1px;background:#aaa;margin:70px 0}.dot{color:#b2693d}footer{font-size:12px;border-top:1px solid #ccc7bb}
</style></head><body><header>{{.Title}}</header><main>` + body + `</main><footer>© {{.Title}}</footer></body></html>`))
		var buf bytes.Buffer
		_ = t.Execute(&buf, c)
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if r.URL.Path != "/" {
				http.NotFound(w, r)
				return
			}
			if r.Method != "GET" && r.Method != "HEAD" {
				w.WriteHeader(405)
				return
			}
			w.Header().Set("Content-Type", "text/html; charset=utf-8")
			if r.Method == "GET" {
				_, _ = w.Write(buf.Bytes())
			}
		})
	}
	if ValidTemplate(c.Template) {
		return http.FileServer(http.Dir(TemplateRoot(c.Template)))
	}
	return http.NotFoundHandler()
}
