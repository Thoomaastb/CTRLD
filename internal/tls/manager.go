// Package tls verwaltet TLS-Zertifikate für CTRLD.
// Unterstützt Self-Signed-Zertifikate für lokale Deployments.
// Let's Encrypt-Integration ist für v2.x geplant.
package tls

import (
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"fmt"
	"math/big"
	"net"
	"os"
	"path/filepath"
	"time"
)

// CertInfo enthält Informationen über ein TLS-Zertifikat.
type CertInfo struct {
	Subject    string    `json:"subject"`
	Issuer     string    `json:"issuer"`
	NotBefore  time.Time `json:"not_before"`
	NotAfter   time.Time `json:"not_after"`
	DNSNames   []string  `json:"dns_names"`
	IsSelfSigned bool    `json:"is_self_signed"`
	DaysLeft   int       `json:"days_left"`
	IsExpired  bool      `json:"is_expired"`
	IsValid    bool      `json:"is_valid"`
}

// Manager verwaltet TLS-Zertifikate.
type Manager struct {
	certDir string
	certFile string
	keyFile  string
}

// NewManager erstellt einen neuen TLS-Manager.
func NewManager(certDir string) *Manager {
	return &Manager{
		certDir:  certDir,
		certFile: filepath.Join(certDir, "cert.pem"),
		keyFile:  filepath.Join(certDir, "key.pem"),
	}
}

// GenerateSelfSigned erstellt ein selbst-signiertes Zertifikat.
func (m *Manager) GenerateSelfSigned(hosts []string, validDays int) error {
	if err := os.MkdirAll(m.certDir, 0700); err != nil {
		return fmt.Errorf("tls: verzeichnis erstellen fehlgeschlagen: %w", err)
	}

	// ECDSA P-256 Key
	priv, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		return fmt.Errorf("tls: key generieren fehlgeschlagen: %w", err)
	}

	// Zertifikat-Template
	serial, err := rand.Int(rand.Reader, new(big.Int).Lsh(big.NewInt(1), 128))
	if err != nil {
		return fmt.Errorf("tls: serial generieren fehlgeschlagen: %w", err)
	}

	now := time.Now()
	template := x509.Certificate{
		SerialNumber: serial,
		Subject: pkix.Name{
			Organization: []string{"CTRLD"},
			CommonName:   "CTRLD Self-Signed Certificate",
		},
		NotBefore:             now,
		NotAfter:              now.Add(time.Duration(validDays) * 24 * time.Hour),
		KeyUsage:              x509.KeyUsageKeyEncipherment | x509.KeyUsageDigitalSignature,
		ExtKeyUsage:           []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth},
		BasicConstraintsValid: true,
	}

	// Hosts hinzufügen
	for _, h := range hosts {
		if ip := net.ParseIP(h); ip != nil {
			template.IPAddresses = append(template.IPAddresses, ip)
		} else {
			template.DNSNames = append(template.DNSNames, h)
		}
	}

	// Zertifikat erstellen
	certDER, err := x509.CreateCertificate(rand.Reader, &template, &template, &priv.PublicKey, priv)
	if err != nil {
		return fmt.Errorf("tls: zertifikat erstellen fehlgeschlagen: %w", err)
	}

	// Cert PEM schreiben
	certFile, err := os.OpenFile(m.certFile, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0644)
	if err != nil {
		return fmt.Errorf("tls: cert datei öffnen fehlgeschlagen: %w", err)
	}
	defer certFile.Close()
	if err := pem.Encode(certFile, &pem.Block{Type: "CERTIFICATE", Bytes: certDER}); err != nil {
		return fmt.Errorf("tls: cert schreiben fehlgeschlagen: %w", err)
	}

	// Key PEM schreiben (0600 — nur root lesbar)
	keyFile, err := os.OpenFile(m.keyFile, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0600)
	if err != nil {
		return fmt.Errorf("tls: key datei öffnen fehlgeschlagen: %w", err)
	}
	defer keyFile.Close()

	keyDER, err := x509.MarshalECPrivateKey(priv)
	if err != nil {
		return fmt.Errorf("tls: key marshaling fehlgeschlagen: %w", err)
	}
	if err := pem.Encode(keyFile, &pem.Block{Type: "EC PRIVATE KEY", Bytes: keyDER}); err != nil {
		return fmt.Errorf("tls: key schreiben fehlgeschlagen: %w", err)
	}

	return nil
}

// GetCertInfo gibt Informationen über das aktuelle Zertifikat zurück.
func (m *Manager) GetCertInfo() (*CertInfo, error) {
	certPEM, err := os.ReadFile(m.certFile)
	if err != nil {
		return nil, fmt.Errorf("tls: zertifikat nicht gefunden: %w", err)
	}

	block, _ := pem.Decode(certPEM)
	if block == nil {
		return nil, fmt.Errorf("tls: ungültiges PEM-Format")
	}

	cert, err := x509.ParseCertificate(block.Bytes)
	if err != nil {
		return nil, fmt.Errorf("tls: zertifikat parsen fehlgeschlagen: %w", err)
	}

	now := time.Now()
	daysLeft := int(cert.NotAfter.Sub(now).Hours() / 24)

	return &CertInfo{
		Subject:      cert.Subject.CommonName,
		Issuer:       cert.Issuer.CommonName,
		NotBefore:    cert.NotBefore,
		NotAfter:     cert.NotAfter,
		DNSNames:     cert.DNSNames,
		IsSelfSigned: cert.Subject.String() == cert.Issuer.String(),
		DaysLeft:     daysLeft,
		IsExpired:    now.After(cert.NotAfter),
		IsValid:      now.After(cert.NotBefore) && !now.After(cert.NotAfter),
	}, nil
}

// HasCertificate prüft ob ein Zertifikat vorhanden ist.
func (m *Manager) HasCertificate() bool {
	_, err := os.Stat(m.certFile)
	return err == nil
}

// CertFile gibt den Pfad zur Zertifikat-Datei zurück.
func (m *Manager) CertFile() string { return m.certFile }

// KeyFile gibt den Pfad zur Key-Datei zurück.
func (m *Manager) KeyFile() string { return m.keyFile }
