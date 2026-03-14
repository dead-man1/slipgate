package dnsrouter

import (
	"fmt"
	"log"
	"net"
	"strings"
	"sync"

	"github.com/anonvector/slipgate/internal/config"
)

// Router is a pure Go DNS forwarder that routes queries by domain.
type Router struct {
	listenAddr string
	routes     map[string]string // domain → backend address (host:port)
	defaultDst string
	mu         sync.RWMutex
	conn       *net.UDPConn
}

// New creates a new DNS router.
func New(listenAddr string) *Router {
	return &Router{
		listenAddr: listenAddr,
		routes:     make(map[string]string),
	}
}

// AddRoute maps a domain to a backend address.
func (r *Router) AddRoute(domain, backend string) {
	r.mu.Lock()
	defer r.mu.Unlock()
	// Normalize: ensure trailing dot
	if !strings.HasSuffix(domain, ".") {
		domain += "."
	}
	r.routes[strings.ToLower(domain)] = backend
}

// SetDefault sets the default backend for unmatched queries.
func (r *Router) SetDefault(backend string) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.defaultDst = backend
}

// ListenAndServe starts the DNS router.
func (r *Router) ListenAndServe() error {
	addr, err := net.ResolveUDPAddr("udp", r.listenAddr)
	if err != nil {
		return fmt.Errorf("resolve listen addr: %w", err)
	}

	r.conn, err = net.ListenUDP("udp", addr)
	if err != nil {
		return fmt.Errorf("listen udp: %w", err)
	}
	defer r.conn.Close()

	log.Printf("DNS router listening on %s", r.listenAddr)

	buf := make([]byte, 4096)
	for {
		n, clientAddr, err := r.conn.ReadFromUDP(buf)
		if err != nil {
			log.Printf("read error: %v", err)
			continue
		}

		// Copy packet for goroutine
		packet := make([]byte, n)
		copy(packet, buf[:n])

		go r.handleQuery(packet, clientAddr)
	}
}

func (r *Router) handleQuery(packet []byte, clientAddr *net.UDPAddr) {
	// Extract domain from DNS query
	domain := extractDomain(packet)
	if domain == "" {
		return
	}

	// Find matching backend
	r.mu.RLock()
	backend := r.findBackend(domain)
	r.mu.RUnlock()

	if backend == "" {
		log.Printf("no route for %s", domain)
		return
	}

	// Forward to backend
	resp, err := forward(packet, backend)
	if err != nil {
		log.Printf("forward to %s: %v", backend, err)
		return
	}

	// Send response back to client
	r.conn.WriteToUDP(resp, clientAddr)
}

func (r *Router) findBackend(domain string) string {
	domain = strings.ToLower(domain)
	if !strings.HasSuffix(domain, ".") {
		domain += "."
	}

	// Try exact match first
	if dst, ok := r.routes[domain]; ok {
		return dst
	}

	// Try suffix match (for subdomains)
	for routeDomain, dst := range r.routes {
		if strings.HasSuffix(domain, "."+routeDomain) || domain == routeDomain {
			return dst
		}
	}

	return r.defaultDst
}

func forward(packet []byte, backend string) ([]byte, error) {
	addr, err := net.ResolveUDPAddr("udp", backend)
	if err != nil {
		return nil, err
	}

	conn, err := net.DialUDP("udp", nil, addr)
	if err != nil {
		return nil, err
	}
	defer conn.Close()

	if _, err := conn.Write(packet); err != nil {
		return nil, err
	}

	buf := make([]byte, 4096)
	n, err := conn.Read(buf)
	if err != nil {
		return nil, err
	}

	return buf[:n], nil
}

// Serve loads config and starts the DNS router.
func Serve(cfgInterface interface{}) error {
	cfg, ok := cfgInterface.(*config.Config)
	if !ok {
		return fmt.Errorf("invalid config type")
	}

	r := New(cfg.Listen.Address)

	for _, tunnel := range cfg.Tunnels {
		if !tunnel.Enabled || !tunnel.IsDNSTunnel() {
			continue
		}
		backend := fmt.Sprintf("127.0.0.1:%d", tunnel.Port)
		r.AddRoute(tunnel.Domain, backend)
		log.Printf("route: %s → %s", tunnel.Domain, backend)
	}

	if cfg.Route.Default != "" {
		defaultTunnel := cfg.GetTunnel(cfg.Route.Default)
		if defaultTunnel != nil && defaultTunnel.IsDNSTunnel() {
			r.SetDefault(fmt.Sprintf("127.0.0.1:%d", defaultTunnel.Port))
		}
	}

	return r.ListenAndServe()
}
