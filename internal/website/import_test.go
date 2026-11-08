package website

import (
	"archive/zip"
	"bytes"
	"context"
	"io"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestTemplateCatalogDownload(t *testing.T) {
	if os.Getenv("XUI_TEST_TEMPLATE_DOWNLOAD") != "1" {
		t.Skip("set XUI_TEST_TEMPLATE_DOWNLOAD=1 for a real catalog download")
	}
	t.Setenv("XUI_DB_FOLDER", t.TempDir())
	if err := InstallTemplate(context.Background(), "sb-resume"); err != nil {
		t.Fatal(err)
	}
	if !ValidTemplate("sb-resume") {
		t.Fatal("downloaded template is not installed")
	}
	if _, err := os.Stat(filepath.Join(TemplateRoot("sb-resume"), "LICENSE")); err != nil {
		t.Fatal(err)
	}
}

func templateZIP(t *testing.T, files map[string]string) []byte {
	t.Helper()
	var data bytes.Buffer
	writer := zip.NewWriter(&data)
	for name, content := range files {
		file, err := writer.Create(name)
		if err != nil {
			t.Fatal(err)
		}
		_, _ = io.WriteString(file, content)
	}
	if err := writer.Close(); err != nil {
		t.Fatal(err)
	}
	return data.Bytes()
}

func TestTemplateZIPImportPreservesBuiltAssetsAndLicense(t *testing.T) {
	t.Setenv("XUI_DB_FOLDER", t.TempDir())
	data := templateZIP(t, map[string]string{"project/dist/index.html": "<html>Imported site</html>", "project/dist/css/style.css": "body{color:red}", "project/LICENSE": "MIT license notice"})
	id, err := ImportTemplate(data)
	if err != nil {
		t.Fatal(err)
	}
	if !ValidTemplate(id) {
		t.Fatal("imported template is not selectable")
	}
	for _, name := range []string{"index.html", "css/style.css", "LICENSE"} {
		if _, err := os.Stat(filepath.Join(TemplateRoot(id), name)); err != nil {
			t.Fatal(err)
		}
	}
	w := httptest.NewRecorder()
	TemplateHandler(Config{Template: id}).ServeHTTP(w, httptest.NewRequest("GET", "/css/style.css", nil))
	if w.Code != 200 || w.Body.String() != "body{color:red}" {
		t.Fatalf("asset response %d %q", w.Code, w.Body.String())
	}
}

func TestTemplateZIPRejectsTraversalAndMissingBuild(t *testing.T) {
	t.Setenv("XUI_DB_FOLDER", t.TempDir())
	for _, name := range []string{"../escape", "/absolute", "a/../../escape", "a\\escape", "C:/escape"} {
		if _, err := ImportTemplate(templateZIP(t, map[string]string{"index.html": "ok", name: "unsafe"})); err == nil {
			t.Errorf("accepted unsafe path %q", name)
		}
	}
	if _, err := ImportTemplate(templateZIP(t, map[string]string{"src/index.pug": "source only"})); err == nil || !strings.Contains(err.Error(), "index.html") {
		t.Fatalf("unexpected missing-build error: %v", err)
	}
}

func TestBuiltinTemplateVariantsRenderEscapedContent(t *testing.T) {
	for _, item := range builtinTemplates {
		t.Run(item.ID, func(t *testing.T) {
			w := httptest.NewRecorder()
			TemplateHandler(Config{Template: item.ID, Title: "<script>bad</script>", Description: "Example"}).ServeHTTP(w, httptest.NewRequest("GET", "/", nil))
			if w.Code != 200 || !strings.Contains(w.Body.String(), "&lt;script&gt;") || strings.Contains(w.Body.String(), "<script>bad") {
				t.Fatalf("invalid template output: %d %s", w.Code, w.Body.String())
			}
		})
	}
}
