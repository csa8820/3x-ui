package website

import (
	"crypto/tls"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/mhsanaei/3x-ui/v3/internal/config"
	"golang.org/x/crypto/acme"
	"golang.org/x/crypto/acme/autocert"
)

type TLSSettings struct {
	MinVersion    string `json:"minVersion"`
	MaxVersion    string `json:"maxVersion"`
	ECHServerKeys string `json:"echServerKeys"`
	Certificates  []struct {
		CertificateFile string   `json:"certificateFile"`
		KeyFile         string   `json:"keyFile"`
		Certificate     []string `json:"certificate"`
		Key             []string `json:"key"`
		Usage           string   `json:"usage"`
	} `json:"certificates"`
}

func (s TLSSettings) LoadCertificates() ([]tls.Certificate, error) {
	var result []tls.Certificate
	for _, c := range s.Certificates {
		if c.Usage == "verify" {
			continue
		}
		cert, key := []byte(strings.Join(c.Certificate, "\n")), []byte(strings.Join(c.Key, "\n"))
		var err error
		if c.CertificateFile != "" {
			cert, err = os.ReadFile(c.CertificateFile)
			if err != nil {
				return nil, err
			}
		}
		if c.KeyFile != "" {
			key, err = os.ReadFile(c.KeyFile)
			if err != nil {
				return nil, err
			}
		}
		parsed, err := tls.X509KeyPair(cert, key)
		if err != nil {
			return nil, fmt.Errorf("website TLS certificate: %w", err)
		}
		result = append(result, parsed)
	}
	return result, nil
}

func TLSConfig(s Stream) (*tls.Config, error) {
	c := &tls.Config{MinVersion: tls.VersionTLS12, NextProtos: []string{"h2", "http/1.1"}}
	if s.TLS.MinVersion == "1.3" {
		c.MinVersion = tls.VersionTLS13
	}
	if s.TLS.MaxVersion == "1.2" {
		c.MaxVersion = tls.VersionTLS12
	}
	if s.Website.AutoTLS {
		cache := filepath.Join(config.GetDBFolderPath(), "website-certs")
		if err := os.MkdirAll(cache, 0700); err != nil {
			return nil, err
		}
		manager := &autocert.Manager{
			Prompt: autocert.AcceptTOS, Cache: autocert.DirCache(cache),
			HostPolicy: autocert.HostWhitelist(s.Website.Domain), Email: s.Website.Email,
		}
		c.GetCertificate = manager.GetCertificate
		c.NextProtos = append(c.NextProtos, acme.ALPNProto)
		return c, nil
	}
	certs, err := s.TLS.LoadCertificates()
	if err != nil {
		return nil, err
	}
	if len(certs) == 0 {
		return nil, errors.New("website TLS certificate is missing")
	}
	// Re-read file certificates at handshake time so existing renewal jobs need no restart hook.
	c.GetCertificate = func(hello *tls.ClientHelloInfo) (*tls.Certificate, error) {
		certs, err := s.TLS.LoadCertificates()
		if err != nil {
			return nil, err
		}
		for i := range certs {
			if hello.SupportsCertificate(&certs[i]) == nil {
				return &certs[i], nil
			}
		}
		return nil, errors.New("no website certificate matches the client hello")
	}
	return c, nil
}
