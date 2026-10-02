package client

import (
	"crypto/tls"
	"crypto/x509"
	"fmt"
	"os"
)

// TLSSettings configure certificates for every request.
type TLSSettings struct {
	// CABundle is a PEM file of extra certificate authorities to trust.
	CABundle string
	// CertFile and KeyFile are a client certificate and its private key, in
	// PEM. KeyFile may be empty when the certificate file also holds the key.
	CertFile string
	KeyFile  string
}

// tlsMaterial is what TLSSettings' files hold.
type tlsMaterial struct {
	roots *x509.CertPool // nil trusts the system's roots alone
	certs []tls.Certificate
}

func loadTLS(settings TLSSettings) (tlsMaterial, error) {
	var m tlsMaterial
	if settings.CABundle != "" {
		pem, err := os.ReadFile(settings.CABundle)
		if err != nil {
			return m, fmt.Errorf("reading CA bundle: %w", err)
		}
		roots, err := x509.SystemCertPool()
		if err != nil || roots == nil {
			roots = x509.NewCertPool()
		}
		if !roots.AppendCertsFromPEM(pem) {
			return m, fmt.Errorf("no certificates found in CA bundle %s", settings.CABundle)
		}
		m.roots = roots
	}
	if settings.CertFile != "" {
		keyFile := settings.KeyFile
		if keyFile == "" {
			keyFile = settings.CertFile
		}
		cert, err := tls.LoadX509KeyPair(settings.CertFile, keyFile)
		if err != nil {
			return m, fmt.Errorf("loading client certificate: %w", err)
		}
		m.certs = []tls.Certificate{cert}
	}
	return m, nil
}

// config is a TLS configuration with the material. verify false skips
// checking the server's certificate.
func (m tlsMaterial) config(verify bool) *tls.Config {
	return &tls.Config{
		InsecureSkipVerify: !verify, //nolint:gosec // The user's choice, per request.
		RootCAs:            m.roots,
		Certificates:       m.certs,
	}
}
