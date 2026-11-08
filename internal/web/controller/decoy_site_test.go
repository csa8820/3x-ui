package controller

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"
	"path/filepath"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/mhsanaei/3x-ui/v3/internal/database"
	"github.com/mhsanaei/3x-ui/v3/internal/database/dbtest"
	"github.com/mhsanaei/3x-ui/v3/internal/database/model"
	"github.com/mhsanaei/3x-ui/v3/internal/web/middleware"
)

func TestSiteSaveAcceptsPanelFormEncoding(t *testing.T) {
	dbtest.InitDB(t, filepath.Join(t.TempDir(), "x-ui.db"))
	db := database.GetDB()
	router := gin.New()
	NewDecoySiteController(router.Group("/sites"))
	form := url.Values{"id": {"0"}, "name": {"Browser site"}, "config": {`{"domain":"site.example.com","autoTLS":true,"port":443,"template":"journal"}`}}
	request := httptest.NewRequest(http.MethodPost, "/sites/save", strings.NewReader(form.Encode()))
	request.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	w := httptest.NewRecorder()
	router.ServeHTTP(w, request)
	var response struct {
		Success bool            `json:"success"`
		Msg     string          `json:"msg"`
		Obj     model.DecoySite `json:"obj"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &response); err != nil {
		t.Fatal(err)
	}
	if !response.Success || response.Obj.Id == 0 {
		t.Fatalf("save failed: %s", w.Body.String())
	}
	var stored model.DecoySite
	if err := db.First(&stored, response.Obj.Id).Error; err != nil {
		t.Fatal(err)
	}
	if stored.Name != "Browser site" || !strings.Contains(stored.Config, `"template":"journal"`) {
		t.Fatalf("wrong saved site: %+v", stored)
	}
}

func TestSitePreviewAllowsSameOriginFramesAndDisablesScripts(t *testing.T) {
	router := gin.New()
	router.Use(middleware.SecurityHeadersMiddleware(false))
	NewDecoySiteController(router.Group("/sites"))
	w := httptest.NewRecorder()
	router.ServeHTTP(w, httptest.NewRequest("GET", "/sites/templates/preview/journal/", nil))
	if w.Code != 200 || w.Header().Get("X-Frame-Options") != "SAMEORIGIN" {
		t.Fatalf("preview frame blocked: %d %v", w.Code, w.Header())
	}
	policy := w.Header().Get("Content-Security-Policy")
	if !strings.Contains(policy, "sandbox") || !strings.Contains(policy, "script-src 'none'") {
		t.Fatalf("unsafe preview policy: %s", policy)
	}
}
