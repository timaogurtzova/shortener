package httpserver

import (
	"crypto/tls"
	"crypto/x509"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/timaogurtzova/shortener/internal/config"
)

func TestNewTLSConfigCreatesCertificateForServerHost(t *testing.T) {
	tlsConfig, err := newTLSConfig("short.example:8443")
	require.NoError(t, err)
	require.Len(t, tlsConfig.Certificates, 1)
	assert.Equal(t, uint16(tls.VersionTLS12), tlsConfig.MinVersion)

	certificate, err := x509.ParseCertificate(tlsConfig.Certificates[0].Certificate[0])
	require.NoError(t, err)
	assert.NoError(t, certificate.VerifyHostname("short.example"))
	assert.NoError(t, certificate.VerifyHostname("localhost"))
	assert.NoError(t, certificate.VerifyHostname("127.0.0.1"))
	assert.Contains(t, certificate.ExtKeyUsage, x509.ExtKeyUsageServerAuth)
}

func TestNewServerKeepsConfiguredHTTPSMode(t *testing.T) {
	cfg := &config.Configuration{
		Server: config.ServerConfiguration{
			Address:     "localhost:8080",
			EnableHTTPS: true,
		},
	}

	server := NewServer(cfg, nil)

	assert.True(t, server.enableHTTPS)
}

func TestTLSConfigServesHTTPS(t *testing.T) {
	tlsConfig, err := newTLSConfig("localhost:8443")
	require.NoError(t, err)

	server := httptest.NewUnstartedServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte("secure"))
	}))
	server.TLS = tlsConfig
	server.StartTLS()
	t.Cleanup(server.Close)

	response, err := server.Client().Get(server.URL)
	require.NoError(t, err)
	defer response.Body.Close()

	body, err := io.ReadAll(response.Body)
	require.NoError(t, err)
	assert.Equal(t, http.StatusOK, response.StatusCode)
	assert.Equal(t, "secure", string(body))
}
