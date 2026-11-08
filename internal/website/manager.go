package website

import (
	"context"
	"crypto/tls"
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"net/http"
	"sync"
	"time"

	"golang.org/x/net/http2"
)

type Instance struct {
	Tag    string
	Listen string
	Port   int
	Stream Stream
}

type running struct {
	server      *http.Server
	listener    net.Listener
	transport   interface{ CloseIdleConnections() }
	fingerprint string
	address     string
}

type Manager struct {
	mu        sync.Mutex
	instances map[string]*running
	statuses  map[string]Status
}

type Status struct {
	Running          bool   `json:"running"`
	CertificateReady bool   `json:"certificateReady"`
	Error            string `json:"error"`
}

var defaultManager = NewManager()

func NewManager() *Manager {
	return &Manager{instances: make(map[string]*running), statuses: make(map[string]Status)}
}
func GetManager() *Manager { return defaultManager }

func (m *Manager) Status(tag string) Status { m.mu.Lock(); defer m.mu.Unlock(); return m.statuses[tag] }

func (m *Manager) Ensure(inst Instance) (ensureErr error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	defer func() {
		if ensureErr != nil {
			status := m.statuses[inst.Tag]
			status.Error = ensureErr.Error()
			m.statuses[inst.Tag] = status
		}
	}()
	encoded, _ := json.Marshal(inst)
	fingerprint := string(encoded)
	old := m.instances[inst.Tag]
	if old != nil && old.fingerprint == fingerprint {
		return nil
	}
	if inst.Stream.Website.Mode == "fallback" {
		return m.ensureFallbackLocked(inst, fingerprint)
	}
	tlsConfig, err := TLSConfig(inst.Stream)
	if err != nil {
		return err
	}
	getCert := tlsConfig.GetCertificate
	tlsConfig.GetCertificate = func(hello *tls.ClientHelloInfo) (*tls.Certificate, error) {
		cert, err := getCert(hello)
		// ACME challenge certificates are temporary and do not mean issuance has completed.
		if len(hello.SupportedProtos) == 1 && hello.SupportedProtos[0] == "acme-tls/1" {
			return cert, err
		}
		m.mu.Lock()
		if entry := m.instances[inst.Tag]; entry != nil && entry.fingerprint == fingerprint {
			status := m.statuses[inst.Tag]
			if err != nil {
				status.Error = err.Error()
			} else {
				status.CertificateReady = true
				status.Error = ""
			}
			m.statuses[inst.Tag] = status
		}
		m.mu.Unlock()
		return cert, err
	}
	transport := &http2.Transport{AllowHTTP: true,
		DialTLSContext: func(ctx context.Context, _, _ string, _ *tls.Config) (net.Conn, error) {
			return (&net.Dialer{Timeout: 10 * time.Second}).DialContext(ctx, "unix", BackendAddress(inst.Tag))
		},
	}
	var upstream http.RoundTripper = transport
	var closer interface{ CloseIdleConnections() } = transport
	if inst.Stream.Network == "ws" || inst.Stream.Network == "httpupgrade" {
		h1 := &http.Transport{DialContext: func(ctx context.Context, _, _ string) (net.Conn, error) {
			return (&net.Dialer{Timeout: 10 * time.Second}).DialContext(ctx, "unix", BackendAddress(inst.Tag))
		}}
		upstream, closer = h1, h1
	}
	proxyPath := inst.Stream.XHTTP.Path
	if !IsPathMode(inst.Stream.Website.Mode) {
		proxyPath = ""
	}
	handler := NewHandler(inst.Stream.Website, proxyPath, "http://localhost", upstream)
	server := &http.Server{Handler: handler, TLSConfig: tlsConfig, ReadHeaderTimeout: 10 * time.Second, IdleTimeout: 2 * time.Minute}
	if err := http2.ConfigureServer(server, &http2.Server{}); err != nil {
		return err
	}
	addr := Address(inst.Listen, inst.Port)
	network := "tcp"
	if inst.Stream.Website.Mode == "reality" {
		network, addr = "unix", SiteAddress(inst.Tag)
	}
	var listener net.Listener
	if old != nil && old.address == addr {
		// A new handler/config can use the already-bound socket after shutting down the old server.
		m.removeLocked(inst.Tag)
	}
	listener, err = net.Listen(network, addr)
	if err != nil {
		transport.CloseIdleConnections()
		return fmt.Errorf("website %s cannot listen on %s: %w", inst.Tag, addr, err)
	}
	if old != nil {
		m.removeLocked(inst.Tag)
	}
	entry := &running{server: server, listener: listener, transport: closer, fingerprint: fingerprint, address: addr}
	m.instances[inst.Tag] = entry
	m.statuses[inst.Tag] = Status{Running: true, CertificateReady: !inst.Stream.Website.AutoTLS}
	go func() {
		err := server.ServeTLS(listener, "", "")
		if err != nil && !errors.Is(err, http.ErrServerClosed) {
			m.mu.Lock()
			if m.instances[inst.Tag] == entry {
				delete(m.instances, inst.Tag)
				m.statuses[inst.Tag] = Status{Error: err.Error()}
			}
			m.mu.Unlock()
		}
	}()
	if inst.Stream.Website.AutoTLS {
		go func() {
			_, _ = tlsConfig.GetCertificate(&tls.ClientHelloInfo{ServerName: inst.Stream.Website.Domain,
				SupportedProtos: []string{"h2"}, SupportedVersions: []uint16{tls.VersionTLS13},
				SignatureSchemes: []tls.SignatureScheme{tls.ECDSAWithP256AndSHA256, tls.PSSWithSHA256},
				CipherSuites:     []uint16{tls.TLS_AES_128_GCM_SHA256}, SupportedCurves: []tls.CurveID{tls.CurveP256}})
		}()
	}
	return nil
}

func (m *Manager) removeLocked(tag string) {
	if inst := m.instances[tag]; inst != nil {
		_ = inst.server.Close()
		if inst.transport != nil {
			inst.transport.CloseIdleConnections()
		}
		delete(m.instances, tag)
	}
	delete(m.statuses, tag)
}

func (m *Manager) Remove(tag string) { m.mu.Lock(); defer m.mu.Unlock(); m.removeLocked(tag) }
func (m *Manager) StopAll() {
	m.mu.Lock()
	defer m.mu.Unlock()
	for tag := range m.instances {
		m.removeLocked(tag)
	}
}

func (m *Manager) Reconcile(desired []Instance) error {
	keep := make(map[string]bool, len(desired))
	for _, inst := range desired {
		keep[inst.Tag] = true
	}
	m.Prune(keep)
	var result error
	for _, inst := range desired {
		result = errors.Join(result, m.Ensure(inst))
	}
	return result
}

func (m *Manager) Prune(keep map[string]bool) {
	m.mu.Lock()
	defer m.mu.Unlock()
	for tag := range m.statuses {
		if !keep[tag] {
			m.removeLocked(tag)
		}
	}
}

func (m *Manager) ensureFallbackLocked(inst Instance, fingerprint string) error {
	addr := SiteAddress(inst.Tag)
	if old := m.instances[inst.Tag]; old != nil {
		m.removeLocked(inst.Tag)
	}
	listener, err := net.Listen("unix", addr)
	if err != nil {
		return err
	}
	server := &http.Server{Handler: NewHandler(inst.Stream.Website, "", "http://localhost", http.DefaultTransport), ReadHeaderTimeout: 10 * time.Second}
	entry := &running{server: server, listener: listener, fingerprint: fingerprint, address: addr}
	m.instances[inst.Tag] = entry
	m.statuses[inst.Tag] = Status{Running: true, CertificateReady: true}
	go func() { _ = server.Serve(listener) }()
	return nil
}
