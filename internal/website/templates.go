package website

import (
	"bytes"
	"embed"
	"html/template"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/mhsanaei/3x-ui/v3/internal/config"
)

type TemplateInfo struct {
	ID        string `json:"id"`
	Name      string `json:"name"`
	Category  string `json:"category"`
	Source    string `json:"source"`
	License   string `json:"license"`
	Installed bool   `json:"installed"`
}

type builtinTemplate struct {
	TemplateInfo
	Title       string
	Description string
}

// DefaultTemplate also serves sites saved before a template was chosen.
const DefaultTemplate = "cloud"

//go:embed sitetemplates
var templateFS embed.FS

var builtinPages = template.Must(template.ParseFS(templateFS, "sitetemplates/*.html", "sitetemplates/base.css"))

var builtinTemplates = []builtinTemplate{
	{TemplateInfo{ID: "cloud", Name: "云衡科技", Category: "科技云服务"}, "云衡科技", "云服务器、云数据库、对象存储、CDN 等 200 余款产品，按需购买，开箱即用。"},
	{TemplateInfo{ID: "group", Name: "恒岳集团", Category: "集团制造业"}, "恒岳集团", "业务涵盖高端装备、新能源、新材料与智慧物流的综合性产业集团，坚持自主创新与绿色发展，服务全球 130 多个国家和地区。"},
	{TemplateInfo{ID: "finance", Name: "汇岳资本", Category: "金融投资"}, "汇岳资本", "主要投资于高等级信用债与利率债，力争在控制回撤的前提下获取稳健收益。认购期内认购费率享 1 折优惠。"},
	{TemplateInfo{ID: "legal", Name: "衡正律师事务所", Category: "专业服务"}, "衡正律师事务所", "在公司并购、资本市场、争议解决、知识产权与合规等领域，为境内外企业客户提供法律服务。在北京、上海、深圳、香港设有办公室。"},
	{TemplateInfo{ID: "academy", Name: "知行学院", Category: "教育培训"}, "知行学院", "编程、设计、数据分析、产品运营等 1,100 余门课程，每门都有配套练习和助教答疑。"},
	{TemplateInfo{ID: "brand", Name: "拾光生活", Category: "消费品牌"}, "拾光生活", "源自自然的生活方式品牌。我们挑选天然材料，与各地手艺人合作，制作温暖、耐用、好看的日常用品。"},
	{TemplateInfo{ID: "news", Name: "观澜资讯", Category: "新闻资讯门户"}, "观澜资讯", "调研历时半年，记者走访企业、园区与社区，记录产业升级背后的人与故事，呈现城市转型中的机遇与挑战。"},
	{TemplateInfo{ID: "docs", Name: "星枢开发者文档", Category: "开发者文档"}, "星枢", "本指南将帮助你在 5 分钟内完成 SDK 安装与初始化，并发起第一个 API 请求。"},
}

func TemplatesDir() string          { return filepath.Join(config.GetDBFolderPath(), "site-templates") }
func TemplateRoot(id string) string { return filepath.Join(TemplatesDir(), id) }

func Catalog() []TemplateInfo {
	result := make([]TemplateInfo, 0, len(builtinTemplates))
	for _, item := range builtinTemplates {
		info := item.TemplateInfo
		info.Source, info.Installed = "内置模板", true
		result = append(result, info)
	}
	entries, _ := os.ReadDir(TemplatesDir())
	for _, entry := range entries {
		if entry.IsDir() && strings.HasPrefix(entry.Name(), "custom-") && templateInstalled(entry.Name()) {
			result = append(result, TemplateInfo{ID: entry.Name(), Name: strings.TrimPrefix(entry.Name(), "custom-"), Category: "导入模板", Source: "ZIP", Installed: true})
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

func findBuiltin(id string) (builtinTemplate, bool) {
	for _, item := range builtinTemplates {
		if item.ID == id {
			return item, true
		}
	}
	return builtinTemplate{}, false
}

func TemplateHandler(c Config) http.Handler {
	id := c.Template
	if id == "" {
		id = DefaultTemplate
	}
	if item, ok := findBuiltin(id); ok {
		data := struct{ Title, Description, Domain, Year string }{c.Title, c.Description, c.Domain, strconv.Itoa(time.Now().Year())}
		if data.Title == "" {
			data.Title = item.Title
		}
		if data.Description == "" {
			data.Description = item.Description
		}
		if data.Domain == "" {
			data.Domain = "example.com"
		}
		var buf bytes.Buffer
		if err := builtinPages.ExecuteTemplate(&buf, id+".html", data); err != nil {
			return http.NotFoundHandler()
		}
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if r.URL.Path != "/" && r.URL.Path != "/index.html" {
				http.NotFound(w, r)
				return
			}
			if r.Method != http.MethodGet && r.Method != http.MethodHead {
				w.Header().Set("Allow", "GET, HEAD")
				http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
				return
			}
			w.Header().Set("Content-Type", "text/html; charset=utf-8")
			w.Header().Set("X-Content-Type-Options", "nosniff")
			if r.Method == http.MethodGet {
				_, _ = w.Write(buf.Bytes())
			}
		})
	}
	if ValidTemplate(id) {
		return http.FileServer(http.Dir(TemplateRoot(id)))
	}
	return http.NotFoundHandler()
}
