package website_test

import (
	"bytes"
	"context"
	"crypto/ecdh"
	"crypto/rand"
	"crypto/sha256"
	"crypto/tls"
	"crypto/x509"
	"encoding/base64"
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
)

func TestWebsiteSharedProtocols(t *testing.T) {
	seed := httptest.NewTLSServer(http.NotFoundHandler())
	cert := seed.TLS.Certificates[0]
	seed.Close()
	key, err := x509.MarshalPKCS8PrivateKey(cert.PrivateKey)
	if err != nil {
		t.Fatal(err)
	}
	certPEM := string(pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: cert.Certificate[0]}))
	keyPEM := string(pem.EncodeToMemory(&pem.Block{Type: "PRIVATE KEY", Bytes: key}))
	hash := sha256.Sum256(cert.Certificate[0])
	echo := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { data, _ := io.ReadAll(r.Body); _, _ = w.Write(data) }))
	defer echo.Close()
	_, echoPort, _ := net.SplitHostPort(strings.TrimPrefix(echo.URL, "http://"))
	for _, tc := range []struct{ network, security, mode string }{
		{"ws", "tls", "path"}, {"grpc", "tls", "path"}, {"httpupgrade", "tls", "path"}, {"tcp", "tls", "fallback"}, {"tcp", "reality", "reality"},
	} {
		t.Run(tc.network+"-"+tc.security, func(t *testing.T) {
			listener, err := net.Listen("tcp", "127.0.0.1:0")
			if err != nil {
				t.Fatal(err)
			}
			port := listener.Addr().(*net.TCPAddr).Port
			_ = listener.Close()
			stream := map[string]any{"network": tc.network, "security": tc.security,
				"tlsSettings": map[string]any{"alpn": []string{"http/1.1"}, "certificates": []any{map[string]any{"certificate": []string{certPEM}, "key": []string{keyPEM}}}},
				"website":     map[string]any{"enabled": true, "mode": tc.mode, "domain": "example.com", "template": "news", "title": "Shared site"}}
			clientStream := map[string]any{"network": tc.network, "security": tc.security, "tlsSettings": map[string]any{"serverName": "example.com", "alpn": []string{"http/1.1"}, "pinnedPeerCertSha256": hex.EncodeToString(hash[:])}}
			switch tc.network {
			case "grpc":
				stream["grpcSettings"] = map[string]any{"serviceName": "connect-test"}
				clientStream["grpcSettings"] = stream["grpcSettings"]
				clientStream["tlsSettings"].(map[string]any)["alpn"] = []string{"h2"}
			case "ws", "httpupgrade":
				stream[tc.network+"Settings"] = map[string]any{"path": "/connect-test/"}
				clientStream[tc.network+"Settings"] = stream[tc.network+"Settings"]
			}
			if tc.security == "reality" {
				private, err := ecdh.X25519().GenerateKey(rand.Reader)
				if err != nil {
					t.Fatal(err)
				}
				stream["realitySettings"] = map[string]any{"privateKey": base64.RawURLEncoding.EncodeToString(private.Bytes()), "serverNames": []string{"example.com"}, "shortIds": []string{"abcd"}}
				clientStream["realitySettings"] = map[string]any{"serverName": "example.com", "fingerprint": "chrome", "password": base64.RawURLEncoding.EncodeToString(private.PublicKey().Bytes()), "shortId": "abcd"}
				delete(clientStream, "tlsSettings")
			}
			raw, _ := json.Marshal(stream)
			ib := &model.Inbound{Tag: fmt.Sprintf("shared-%d", port), Protocol: model.VLESS, Listen: "127.0.0.1", Port: port, StreamSettings: string(raw), Settings: `{"decryption":"none","clients":[{"id":"11111111-2222-4333-8444-555555555555"}]}`}
			serverJSON, _ := json.Marshal(map[string]any{"log": map[string]any{"loglevel": "error"}, "inbounds": []any{ib.GenXrayInboundConfig()}, "outbounds": []any{map[string]any{"protocol": "freedom", "settings": map[string]any{"finalRules": []any{map[string]any{"action": "allow"}}}}}})
			server, err := core.StartInstance("json", serverJSON)
			if err != nil {
				t.Fatal(err)
			}
			defer server.Close()
			mgr := website.NewManager()
			defer mgr.StopAll()
			site, _ := website.Parse(string(raw))
			if err := mgr.Ensure(website.Instance{Tag: ib.Tag, Listen: ib.Listen, Port: port, Stream: site}); err != nil {
				t.Fatal(err)
			}
			webTransport := &http.Transport{TLSClientConfig: &tls.Config{InsecureSkipVerify: true, ServerName: "example.com"}}
			defer webTransport.CloseIdleConnections()
			response, err := (&http.Client{Timeout: 15 * time.Second, Transport: webTransport}).Get(fmt.Sprintf("https://127.0.0.1:%d/", port))
			if err != nil {
				t.Fatal(err)
			}
			body, _ := io.ReadAll(response.Body)
			_ = response.Body.Close()
			if response.StatusCode != 200 || !strings.Contains(string(body), "Shared site") {
				t.Fatalf("website response %d %s", response.StatusCode, body)
			}
			clientJSON, _ := json.Marshal(map[string]any{"log": map[string]any{"loglevel": "error"}, "outbounds": []any{map[string]any{"protocol": "vless", "settings": map[string]any{"vnext": []any{map[string]any{"address": "127.0.0.1", "port": port, "users": []any{map[string]any{"id": "11111111-2222-4333-8444-555555555555", "encryption": "none"}}}}}, "streamSettings": clientStream}}})
			client, err := core.StartInstance("json", clientJSON)
			if err != nil {
				t.Fatal(err)
			}
			defer client.Close()
			transport := &http.Transport{DialContext: func(ctx context.Context, _, _ string) (net.Conn, error) {
				p, _ := xnet.PortFromString(echoPort)
				return core.Dial(ctx, client, xnet.TCPDestination(xnet.LocalHostIP, p))
			}}
			defer transport.CloseIdleConnections()
			payload := bytes.Repeat([]byte("shared-website-proxy-test"), 4096)
			response, err = (&http.Client{Timeout: 15 * time.Second, Transport: transport}).Post(echo.URL, "application/octet-stream", bytes.NewReader(payload))
			if err != nil {
				t.Fatal(err)
			}
			body, err = io.ReadAll(response.Body)
			_ = response.Body.Close()
			if err != nil || !bytes.Equal(body, payload) {
				t.Fatalf("proxy transfer received %d of %d bytes: %v", len(body), len(payload), err)
			}
		})
	}
}
