package sub

import (
	"net/url"
	"strings"
	"testing"
)

func TestWebsiteSubscriptionKeepsPublicTLSAndStripsWebsiteMetadata(t *testing.T) {
	raw := `{"network":"xhttp","security":"tls","xhttpSettings":{"path":"/connect-test/","mode":"auto"},"tlsSettings":{"serverName":"site.example.com"},"website":{"enabled":true,"domain":"site.example.com","autoTLS":true,"email":"private@example.com"}}`
	s := &SubService{}
	u, err := url.Parse(s.genVlessLink(shareLinkInbound(raw), "user"))
	if err != nil {
		t.Fatal(err)
	}
	if u.Host != "203.0.113.1:443" || u.Query().Get("security") != "tls" || u.Query().Get("path") != "/connect-test/" || u.Query().Get("sni") != "site.example.com" {
		t.Fatalf("wrong public share endpoint: %s", u)
	}
	data := (&SubJsonService{}).streamData(raw, "user")
	if data["website"] != nil {
		t.Fatal("private website settings leaked into JSON subscription")
	}
	if data["security"] != "tls" || !strings.Contains(u.String(), "type=xhttp") {
		t.Fatal("subscription lost XHTTP + TLS")
	}
}
