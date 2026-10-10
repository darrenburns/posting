package client

import (
	"crypto/tls"
	"crypto/x509"
	"encoding/pem"
	"errors"
	"fmt"
	"os"
	"strings"

	"github.com/youmark/pkcs8"
)

// TLSSettings configure certificates for every request.
type TLSSettings struct {
	// CABundle is a PEM file of extra certificate authorities to trust.
	CABundle string
	// CertFile and KeyFile are a client certificate and its private key, in
	// PEM. KeyFile may be empty when the certificate file also holds the key.
	CertFile string
	KeyFile  string
	// Password decrypts an encrypted private key.
	Password string
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
		cert, err := loadClientCertificate(settings)
		if err != nil {
			return m, fmt.Errorf("loading client certificate: %w", err)
		}
		m.certs = []tls.Certificate{cert}
	}
	return m, nil
}

func loadClientCertificate(settings TLSSettings) (tls.Certificate, error) {
	certPEM, err := os.ReadFile(settings.CertFile)
	if err != nil {
		return tls.Certificate{}, err
	}
	keyPEM := certPEM
	if settings.KeyFile != "" {
		if keyPEM, err = os.ReadFile(settings.KeyFile); err != nil {
			return tls.Certificate{}, err
		}
	}
	block := privateKeyBlock(keyPEM)
	if block != nil && isEncrypted(block) {
		if settings.Password == "" {
			return tls.Certificate{}, errors.New("the private key is encrypted: set ssl.password")
		}
		if block, err = decryptKey(block, []byte(settings.Password)); err != nil {
			return tls.Certificate{}, err
		}
		keyPEM = pem.EncodeToMemory(block)
	}
	return tls.X509KeyPair(certPEM, keyPEM)
}

// privateKeyBlock is the first private key in PEM data, which may also hold
// certificates.
func privateKeyBlock(data []byte) *pem.Block {
	for {
		block, rest := pem.Decode(data)
		if block == nil {
			return nil
		}
		if strings.HasSuffix(block.Type, "PRIVATE KEY") {
			return block
		}
		data = rest
	}
}

// isEncrypted reports whether a key is encrypted, as PKCS #8 or in
// OpenSSL's legacy format (a Proc-Type header on an RSA or EC key).
func isEncrypted(block *pem.Block) bool {
	return block.Type == "ENCRYPTED PRIVATE KEY" || x509.IsEncryptedPEMBlock(block) //nolint:staticcheck // Detecting legacy keys, not producing them.
}

// decryptKey returns an encrypted key block decrypted. Like OpenSSL, it
// accepts both encrypted formats, so keys that worked in Posting 2 work here.
func decryptKey(block *pem.Block, password []byte) (*pem.Block, error) {
	if block.Type == "ENCRYPTED PRIVATE KEY" {
		key, err := pkcs8.ParsePKCS8PrivateKey(block.Bytes, password)
		if err != nil {
			return nil, fmt.Errorf("decrypting the private key (is ssl.password right?): %w", err)
		}
		der, err := x509.MarshalPKCS8PrivateKey(key)
		if err != nil {
			return nil, err
		}
		return &pem.Block{Type: "PRIVATE KEY", Bytes: der}, nil
	}
	der, err := x509.DecryptPEMBlock(block, password) //nolint:staticcheck // Legacy encryption is weak, but these are the user's existing keys.
	if err != nil {
		return nil, fmt.Errorf("decrypting the private key (is ssl.password right?): %w", err)
	}
	return &pem.Block{Type: block.Type, Bytes: der}, nil
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
