// Package tlsconfig создаёт общую TLS-конфигурацию для HTTP- и gRPC-серверов.
package tlsconfig

import (
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"fmt"
	"math/big"
	"net"
	"time"
)

const certificateValidity = 365 * 24 * time.Hour

// New создаёт TLS-конфигурацию с самоподписанным сертификатом.
// Сертификат подходит для локального запуска; публичному серверу нужен
// сертификат от доверенного центра сертификации.
func New(address string) (*tls.Config, error) {
	privateKey, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		return nil, fmt.Errorf("generate private key: %w", err)
	}

	serialNumberLimit := new(big.Int).Lsh(big.NewInt(1), 128)
	serialNumber, err := rand.Int(rand.Reader, serialNumberLimit)
	if err != nil {
		return nil, fmt.Errorf("generate certificate serial number: %w", err)
	}
	if serialNumber.Sign() == 0 {
		serialNumber.SetInt64(1)
	}

	now := time.Now()
	certificate := &x509.Certificate{
		SerialNumber: serialNumber,
		Subject: pkix.Name{
			Organization: []string{"Shortener"},
		},
		NotBefore:             now.Add(-time.Minute),
		NotAfter:              now.Add(certificateValidity),
		KeyUsage:              x509.KeyUsageDigitalSignature,
		ExtKeyUsage:           []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth},
		BasicConstraintsValid: true,
		DNSNames:              []string{"localhost"},
		IPAddresses:           []net.IP{net.IPv4(127, 0, 0, 1), net.IPv6loopback},
	}
	addAddressHost(certificate, address)

	certificateDER, err := x509.CreateCertificate(
		rand.Reader,
		certificate,
		certificate,
		&privateKey.PublicKey,
		privateKey,
	)
	if err != nil {
		return nil, fmt.Errorf("create certificate: %w", err)
	}

	privateKeyDER, err := x509.MarshalPKCS8PrivateKey(privateKey)
	if err != nil {
		return nil, fmt.Errorf("marshal private key: %w", err)
	}

	certificatePEM := pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: certificateDER})
	privateKeyPEM := pem.EncodeToMemory(&pem.Block{Type: "PRIVATE KEY", Bytes: privateKeyDER})
	tlsCertificate, err := tls.X509KeyPair(certificatePEM, privateKeyPEM)
	if err != nil {
		return nil, fmt.Errorf("load certificate and private key: %w", err)
	}

	return &tls.Config{
		Certificates: []tls.Certificate{tlsCertificate},
		MinVersion:   tls.VersionTLS12,
	}, nil
}

func addAddressHost(certificate *x509.Certificate, address string) {
	host, _, err := net.SplitHostPort(address)
	if err != nil || host == "" {
		return
	}

	if ip := net.ParseIP(host); ip != nil {
		certificate.IPAddresses = append(certificate.IPAddresses, ip)
		return
	}

	certificate.DNSNames = append(certificate.DNSNames, host)
}
