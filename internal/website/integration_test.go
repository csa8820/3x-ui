package website_test

import (
	"bytes"
	"context"
	"crypto/sha256"
	"crypto/tls"
	"crypto/x509"
	"encoding/hex"
	"encoding/json"
	"encoding/pem"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/mhsanaei/3x-ui/v3/internal/database/model"
	"github.com/mhsanaei/3x-ui/v3/internal/website"
	xnet "github.com/xtls/xray-core/common/net"
	"github.com/xtls/xray-core/core"
	_ "github.com/xtls/xray-core/main/distro/all"
)

func TestWebsiteRealXHTTPUploadAndDownload(t *testing.T) {
	seed := httptest.NewTLSServer(http.NotFoundHandler())
	cert := seed.TLS.Certificates[0]
	seed.Close()
	key, err := x509.MarshalPKCS8PrivateKey(cert.PrivateKey)
	if err != nil {
		t.Fatal(err)
	}
	certPEM := string(pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: cert.Certificate[0]}))
	certHash := sha256.Sum256(cert.Certificate[0])
	keyPEM := string(pem.EncodeToMemory(&pem.Block{Type: "PRIVATE KEY", Bytes: key}))
	front, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	port := front.Addr().(*net.TCPAddr).Port
	_ = front.Close()
	stream, _ := json.Marshal(map[string]any{
		"network": "xhttp", "security": "tls", "xhttpSettings": map[string]any{"path": "/connect-test/", "mode": "auto"},
		"tlsSettings": map[string]any{"serverName": "example.com", "certificates": []any{map[string]any{"certificate": []string{certPEM}, "key": []string{keyPEM}}}},
		"website":     map[string]any{"enabled": true, "domain": "example.com", "title": "A working website"},
	})
	ib := &model.Inbound{Tag: fmt.Sprintf("website-integration-%d", port), Listen: "127.0.0.1", Port: port,
		Protocol: model.VLESS, StreamSettings: string(stream), Settings: `{"decryption":"none","clients":[{"id":"11111111-2222-4333-8444-555555555555"}]}`}
	serverConfig, _ := json.Marshal(map[string]any{"log": map[string]any{"loglevel": "error"}, "inbounds": []any{ib.GenXrayInboundConfig()}, "outbounds": []any{map[string]any{"protocol": "freedom", "settings": map[string]any{"finalRules": []any{map[string]any{"action": "allow"}}}}}})
	server, err := core.StartInstance("json", serverConfig)
	if err != nil {
		t.Fatal(err)
	}
	defer server.Close()
	mgr := website.NewManager()
	defer mgr.StopAll()
	site, _ := website.Parse(string(stream))
	if err := mgr.Ensure(website.Instance{Tag: ib.Tag, Listen: ib.Listen, Port: port, Stream: site}); err != nil {
		t.Fatal(err)
	}
	webClient := &http.Client{Timeout: 10 * time.Second, Transport: &http.Transport{TLSClientConfig: &tls.Config{InsecureSkipVerify: true, ServerName: "example.com"}}}
	response, err := webClient.Get(fmt.Sprintf("https://127.0.0.1:%d/", port))
	if err != nil {
		t.Fatal(err)
	}
	page, _ := io.ReadAll(response.Body)
	_ = response.Body.Close()
	if response.StatusCode != 200 || !strings.Contains(string(page), "A working website") {
		t.Fatalf("website response: %d %s", response.StatusCode, page)
	}

	echo := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		payload, err := io.ReadAll(r.Body)
		if err != nil {
			t.Errorf("echo upload: %v", err)
		}
		_, _ = w.Write(payload)
	}))
	defer echo.Close()
	_, echoPort, _ := net.SplitHostPort(strings.TrimPrefix(echo.URL, "http://"))
	for _, mode := range []string{"packet-up", "stream-up", "stream-one"} {
		t.Run(mode, func(t *testing.T) {
			config, _ := json.Marshal(map[string]any{"log": map[string]any{"loglevel": "error"}, "outbounds": []any{map[string]any{
				"protocol": "vless", "settings": map[string]any{"vnext": []any{map[string]any{"address": "127.0.0.1", "port": port,
					"users": []any{map[string]any{"id": "11111111-2222-4333-8444-555555555555", "encryption": "none"}}}}},
				"streamSettings": map[string]any{"network": "xhttp", "security": "tls",
					"tlsSettings":   map[string]any{"serverName": "example.com", "pinnedPeerCertSha256": hex.EncodeToString(certHash[:])},
					"xhttpSettings": map[string]any{"path": "/connect-test/", "mode": mode}},
			}}})
			client, err := core.StartInstance("json", config)
			if err != nil {
				t.Fatal(err)
			}
			defer client.Close()
			transport := &http.Transport{DialContext: func(ctx context.Context, _, _ string) (net.Conn, error) {
				p, _ := xnet.PortFromString(echoPort)
				return core.Dial(ctx, client, xnet.TCPDestination(xnet.LocalHostIP, p))
			}}
			defer transport.CloseIdleConnections()
			payload := bytes.Repeat([]byte("xhttp-upload-and-download\n"), 16384)
			resp, err := (&http.Client{Timeout: 20 * time.Second, Transport: transport}).Post(echo.URL, "application/octet-stream", bytes.NewReader(payload))
			if err != nil {
				t.Fatal(err)
			}
			got, err := io.ReadAll(resp.Body)
			_ = resp.Body.Close()
			if err != nil || !bytes.Equal(got, payload) {
				t.Fatalf("transfer failed: received %d of %d bytes, error %v", len(got), len(payload), err)
			}
		})
	}
	if err := mgr.Reconcile(nil); err != nil {
		t.Fatal(err)
	}
	if mgr.Status(ib.Tag).Running {
		t.Fatal("disabled website still running")
	}
	probe, err := net.Listen("tcp", fmt.Sprintf("127.0.0.1:%d", port))
	if err != nil {
		t.Fatalf("website did not release the public port: %v", err)
	}
	_ = probe.Close()
}
