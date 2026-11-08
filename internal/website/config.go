package website

import (
	"crypto/sha256"
	"crypto/x509"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"net/url"
	"path"
	"path/filepath"
	"runtime"
	"strings"
	"unicode/utf8"

	"github.com/mhsanaei/3x-ui/v3/internal/config"
)

type Config struct {
	Enabled     bool   `json:"enabled"`
	Domain      string `json:"domain"`
	AutoTLS     bool   `json:"autoTLS"`
	Email       string `json:"email"`
	Title       string `json:"title"`
	Description string `json:"description"`
}

type Stream struct {
	Website  Config `json:"website"`
	Network  string `json:"network"`
	Security string `json:"security"`
	XHTTP    struct {
		Path string `json:"path"`
	} `json:"xhttpSettings"`
	TLS TLSSettings `json:"tlsSettings"`
}

func Parse(raw string) (Stream, error) {
	var s Stream
	if raw == "" {
		return s, nil
	}
	err := json.Unmarshal([]byte(raw), &s)
	return s, err
}

func ValidatePath(p string) error {
	if p == "/" || !strings.HasPrefix(p, "/") || !strings.HasSuffix(p, "/") ||
		strings.ContainsAny(p, "?#%\\\r\n\t ") || path.Clean(p)+"/" != p {
		return errors.New("website requires a dedicated XHTTP path such as /connect-a83f/")
	}
	return nil
}

func (s Stream) Validate(protocol string, port int) error {
	if !s.Website.Enabled {
		return nil
	}
	if port < 1 || port > 65535 {
		return errors.New("website port must be between 1 and 65535")
	}
	if runtime.GOOS == "windows" {
		return errors.New("built-in XHTTP websites require Linux or macOS")
	}
	if protocol != "vless" || s.Network != "xhttp" || s.Security != "tls" {
		return errors.New("built-in websites require VLESS + XHTTP + TLS")
	}
	if err := ValidatePath(s.XHTTP.Path); err != nil {
		return err
	}
	u, err := url.Parse("https://" + s.Website.Domain)
	if err != nil || u.Hostname() != s.Website.Domain || !strings.Contains(s.Website.Domain, ".") ||
		net.ParseIP(s.Website.Domain) != nil || strings.ContainsAny(s.Website.Domain, "/:@*?#% \\") {
		return errors.New("website domain must be a DNS hostname without scheme, port or path")
	}
	if utf8.RuneCountInString(s.Website.Title) > 160 || utf8.RuneCountInString(s.Website.Description) > 4000 {
		return errors.New("website title or description is too long")
	}
	if s.Website.AutoTLS && s.TLS.MaxVersion == "1.2" {
		return errors.New("automatic website certificates require TLS 1.3 for domain verification")
	}
	if s.Website.AutoTLS && port != 443 {
		return errors.New("automatic website certificates require public port 443")
	}
	if s.TLS.ECHServerKeys != "" {
		return errors.New("built-in websites do not support ECH server keys")
	}
	if !s.Website.AutoTLS {
		certs, err := s.TLS.LoadCertificates()
		if err != nil {
			return err
		}
		if len(certs) == 0 {
			return errors.New("website requires a TLS certificate or automatic certificates")
		}
		matches := false
		for _, cert := range certs {
			leaf, err := x509.ParseCertificate(cert.Certificate[0])
			if err == nil && leaf.VerifyHostname(s.Website.Domain) == nil {
				matches = true
			}
		}
		if !matches {
			return errors.New("website domain does not match the configured certificate")
		}
	}
	return nil
}

// The abstract Linux socket avoids allocating another TCP port or writing socket files.
func BackendAddress(tag string) string {
	base, _ := filepath.Abs(config.GetDBFolderPath())
	hash := sha256.Sum256([]byte(base + "\x00" + tag))
	name := "3x-ui-site-" + hex.EncodeToString(hash[:12])
	if runtime.GOOS == "linux" {
		return "@" + name
	}
	return filepath.Join("/tmp", name+".sock")
}

func CoreStream(raw string) string {
	var stream map[string]any
	if json.Unmarshal([]byte(raw), &stream) != nil {
		return raw
	}
	delete(stream, "website")
	stream["security"] = "none"
	delete(stream, "tlsSettings")
	// Socket flags for the public TCP listener do not apply to a Unix backend.
	stream["sockopt"] = map[string]any{"trustedXForwardedFor": []string{"X-3x-UI-Website"}}
	delete(stream, "finalmask")
	encoded, _ := json.Marshal(stream)
	return string(encoded)
}

func Address(listen string, port int) string {
	return net.JoinHostPort(listen, fmt.Sprint(port))
}
