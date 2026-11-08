package website

import (
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestWebsiteRoutesAndEscapesContent(t *testing.T) {
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("X-Forwarded-For") != "192.0.2.1" {
			t.Errorf("spoofed forwarding header survived: %q", r.Header.Get("X-Forwarded-For"))
		}
		body, _ := io.ReadAll(r.Body)
		_, _ = w.Write([]byte(r.URL.RequestURI() + ":" + string(body)))
	}))
	defer upstream.Close()
	handler := NewHandler(Config{Title: "<script>alert(1)</script>", Description: "Hello"}, "/api/test/", upstream.URL, http.DefaultTransport)
	for _, tc := range []struct {
		path, want string
		code       int
	}{
		{"/", "&lt;script&gt;alert(1)&lt;/script&gt;", 200},
		{"/missing", "404", 404},
		{"/api/test/session?q=1", "/api/test/session?q=1:upload", 200},
		{"/api/testing/", "404", 404},
	} {
		t.Run(tc.path, func(t *testing.T) {
			r := httptest.NewRequest(http.MethodPost, tc.path, strings.NewReader("upload"))
			r.RemoteAddr = "192.0.2.1:1234"
			r.Header.Set("X-Forwarded-For", "attacker")
			if tc.path == "/" {
				r.Method = http.MethodGet
			}
			w := httptest.NewRecorder()
			handler.ServeHTTP(w, r)
			if w.Code != tc.code || !strings.Contains(w.Body.String(), tc.want) {
				t.Fatalf("got %d %q, want %d containing %q", w.Code, w.Body.String(), tc.code, tc.want)
			}
		})
	}
}

func TestValidateRejectsUnroutableWebsitePaths(t *testing.T) {
	for _, path := range []string{"/", "", "/api", "//api/", "/a/../b/", "/a/?q=1", "/a/%2f/"} {
		if err := ValidatePath(path); err == nil {
			t.Errorf("accepted %q", path)
		}
	}
	if err := ValidatePath("/api/a83f/"); err != nil {
		t.Fatal(err)
	}
}
