package httpserver

import (
	"crypto/tls"

	"github.com/timaogurtzova/shortener/internal/tlsconfig"
)

func newTLSConfig(address string) (*tls.Config, error) {
	return tlsconfig.New(address)
}
