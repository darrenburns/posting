package client

import (
	"context"
	"crypto/ecdsa"
	"crypto/rand"
	"crypto/tls"
	"crypto/x509"
	"encoding/pem"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/youmark/pkcs8"

	"github.com/darrenburns/posting/v3/internal/model"
)

const keyPassword = "posting-test-password"

// clientCertServer is a TLS server that replies 200 with the client
// certificate's common name, or 401 when none was presented. Its
// certificate is written to a CA bundle in dir.
func clientCertServer(t *testing.T, dir string) (url, caBundle string) {
	t.Helper()
	server := httptest.NewUnstartedServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if len(r.TLS.PeerCertificates) == 0 {
			w.WriteHeader(http.StatusUnauthorized)
			return
		}
		w.Write([]byte(r.TLS.PeerCertificates[0].Subject.CommonName))
	}))
	serverCert := selfSigned(t)
	server.TLS = &tls.Config{
		Certificates: []tls.Certificate{serverCert},
		ClientAuth:   tls.RequestClientCert,
	}
	server.StartTLS()
	t.Cleanup(server.Close)
	caBundle = filepath.Join(dir, "ca.pem")
	writePEM(t, caBundle, &pem.Block{Type: "CERTIFICATE", Bytes: serverCert.Certificate[0]})
	return server.URL, caBundle
}

func writePEM(t *testing.T, path string, blocks ...*pem.Block) {
	t.Helper()
	var data []byte
	for _, block := range blocks {
		data = append(data, pem.EncodeToMemory(block)...)
	}
	if err := os.WriteFile(path, data, 0o600); err != nil {
		t.Fatal(err)
	}
}

// clientKeyBlocks are the client certificate and its key in each format
// ssl.key_file may hold.
func clientKeyBlocks(t *testing.T) (cert *pem.Block, keys map[string]*pem.Block) {
	t.Helper()
	pair := selfSignedFor(t, "posting-test-client", nil)
	plain, err := x509.MarshalPKCS8PrivateKey(pair.PrivateKey)
	if err != nil {
		t.Fatal(err)
	}
	encrypted, err := pkcs8.MarshalPrivateKey(pair.PrivateKey, []byte(keyPassword), nil)
	if err != nil {
		t.Fatal(err)
	}
	ec, err := x509.MarshalECPrivateKey(pair.PrivateKey.(*ecdsa.PrivateKey))
	if err != nil {
		t.Fatal(err)
	}
	legacy, err := x509.EncryptPEMBlock(rand.Reader, "EC PRIVATE KEY", ec, []byte(keyPassword), x509.PEMCipherAES256) //nolint:staticcheck // Producing a legacy key to load.
	if err != nil {
		t.Fatal(err)
	}
	return &pem.Block{Type: "CERTIFICATE", Bytes: pair.Certificate[0]}, map[string]*pem.Block{
		"plain":     {Type: "PRIVATE KEY", Bytes: plain},
		"encrypted": {Type: "ENCRYPTED PRIVATE KEY", Bytes: encrypted},
		"legacy":    legacy,
	}
}

func sendTo(t *testing.T, url string, settings TLSSettings) (*model.Response, error) {
	t.Helper()
	req := model.NewRequest()
	req.URL = url
	return NewHTTP("", settings).Send(context.Background(), Call{Request: req})
}

func TestHTTPPresentsClientCertificatesWithEncryptedKeys(t *testing.T) {
	dir := t.TempDir()
	url, caBundle := clientCertServer(t, dir)
	cert, keys := clientKeyBlocks(t)
	certFile := filepath.Join(dir, "client.pem")
	writePEM(t, certFile, cert)

	for _, tc := range []struct {
		name, key, password string
		combined            bool
	}{
		{name: "unencrypted key", key: "plain"},
		{name: "unencrypted key, password ignored", key: "plain", password: keyPassword},
		{name: "PKCS #8 encrypted key", key: "encrypted", password: keyPassword},
		{name: "legacy encrypted key", key: "legacy", password: keyPassword},
		{name: "combined PEM with encrypted key", key: "encrypted", password: keyPassword, combined: true},
		{name: "combined PEM with legacy encrypted key", key: "legacy", password: keyPassword, combined: true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			settings := TLSSettings{CABundle: caBundle, CertFile: certFile, Password: tc.password}
			if tc.combined {
				settings.CertFile = filepath.Join(t.TempDir(), "combined.pem")
				writePEM(t, settings.CertFile, cert, keys[tc.key])
			} else {
				settings.KeyFile = filepath.Join(t.TempDir(), "client.key")
				writePEM(t, settings.KeyFile, keys[tc.key])
			}
			resp, err := sendTo(t, url, settings)
			if err != nil {
				t.Fatalf("send: %v", err)
			}
			if resp.StatusCode != http.StatusOK || string(resp.Body) != "posting-test-client" {
				t.Errorf("server saw %d %q, want 200 from posting-test-client", resp.StatusCode, resp.Body)
			}
		})
	}
}

func TestHTTPExplainsEncryptedKeyFailures(t *testing.T) {
	dir := t.TempDir()
	url, caBundle := clientCertServer(t, dir)
	cert, keys := clientKeyBlocks(t)
	certFile := filepath.Join(dir, "client.pem")
	writePEM(t, certFile, cert)

	for _, tc := range []struct{ name, key, password, want string }{
		{name: "missing password", key: "encrypted", want: "the private key is encrypted: set ssl.password"},
		{name: "missing password, legacy", key: "legacy", want: "the private key is encrypted: set ssl.password"},
		{name: "wrong password", key: "encrypted", password: "wrong", want: "is ssl.password right?"},
		{name: "wrong password, legacy", key: "legacy", password: "wrong", want: "is ssl.password right?"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			keyFile := filepath.Join(t.TempDir(), "client.key")
			writePEM(t, keyFile, keys[tc.key])
			_, err := sendTo(t, url, TLSSettings{CABundle: caBundle, CertFile: certFile, KeyFile: keyFile, Password: tc.password})
			if err == nil || !strings.Contains(err.Error(), tc.want) {
				t.Errorf("error = %v, want it to contain %q", err, tc.want)
			}
		})
	}
}
