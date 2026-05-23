package sensors

import (
	"context"
	"crypto/tls"
	"fmt"
	"net"
	"time"
)

func init() {
	RegisterDefault("tls", func(args ...string) (SensorReader, error) {
		if len(args) == 0 {
			return nil, fmt.Errorf("domain must be provided")
		}
		domain := args[0]
		return NewTlsSensor(domain), nil
	})
}

// TlsSensor implements SensorReader for TLS certificate monitoring.
type TlsSensor struct {
	domain  string
	results map[string]string
}

// NewTlsSensor creates a new TLS sensor for the given domain.
func NewTlsSensor(domain string) *TlsSensor {
	return &TlsSensor{
		domain:  domain,
		results: make(map[string]string),
	}
}

// Kind returns the sensor type identifier.
func (s *TlsSensor) Kind() string {
	return "tls"
}

// Execute connects to the domain's HTTPS endpoint, inspects the certificate,
// and populates the results map.
func (s *TlsSensor) Execute(ctx context.Context, args []string) error {
	domain := s.domain
	if len(args) > 0 && args[0] != "" {
		domain = args[0]
	}

	dialer := &tls.Dialer{
		NetDialer: &net.Dialer{Timeout: 10 * time.Second},
		Config: &tls.Config{
			InsecureSkipVerify: true, // we want the cert data, not validation
		},
	}

	addr := net.JoinHostPort(domain, "443")
	conn, err := dialer.DialContext(ctx, "tcp", addr)
	if err != nil {
		return fmt.Errorf("connect to %s: %w", addr, err)
	}
	defer conn.Close()

	// dialer returns net.Conn, but the actual type is *tls.Conn
	tlsConn, ok := conn.(*tls.Conn)
	if !ok {
		return fmt.Errorf("unexpected connection type")
	}

	state := tlsConn.ConnectionState()
	cert := state.PeerCertificates[0]

	now := time.Now()

	results := make(map[string]string)
	results["domain"] = domain
	results["issuer"] = cert.Issuer.CommonName
	results["subject"] = cert.Subject.CommonName

	results["expires"] = cert.NotAfter.Format(time.RFC3339)

	remaining := cert.NotAfter.Sub(now)
	if remaining < 0 {
		results["expired"] = "true"
		results["expired_days"] = fmt.Sprintf("%d", int(remaining.Hours())/24)
	} else {
		results["expired"] = "false"
		results["days_remaining"] = fmt.Sprintf("%d", int(remaining.Hours())/24)
	}

	// issuer details
	if len(cert.Issuer.Organization) > 0 {
		results["issuer_org"] = cert.Issuer.Organization[0]
	}
	if len(cert.Issuer.OrganizationalUnit) > 0 {
		results["issuer_ou"] = cert.Issuer.OrganizationalUnit[0]
	}

	// algorithm info
	results["signature_algorithm"] = cert.SignatureAlgorithm.String()

	s.results = results
	return nil
}

// Results returns the collected metrics.
func (s *TlsSensor) Results() map[string]string {
	return s.results
}
