// Package apkbrand rewrites the launcher branding (application label and icon) of a
// prebuilt Android APK and re-signs the result with the APK Signature Scheme v2.
//
// The launcher reads the application label and the launcher icon from the installed
// package, therefore the branding of a deployment can only be changed by rewriting the
// resources of the APK itself. The server ships a generic APK and applies the branding
// of the current installation (system name and favicon) right before the download.
package apkbrand

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"sync"

	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"math/big"
	"time"
)

// Branding carries the launcher branding that is written into the APK.
type Branding struct {
	// Label is the launcher label, e.g. "Acme BYOG".
	Label string
	// IconPNG is the launcher icon encoded as PNG.
	IconPNG []byte
}

// Signer holds the key material used to re-sign a rebranded APK.
type Signer struct {
	PrivateKey *rsa.PrivateKey
	Cert       *x509.Certificate
}

// titlePattern matches the title of the bundled web interface.
var titlePattern = regexp.MustCompile(`(?i)<title>.*?</title>`)

// Rebrand applies the branding to the given APK and re-signs it.
//
// The base APK must be a valid, signed archive. Entries that belong to the JAR
// signature of the base APK are dropped because they no longer match the rewritten
// content; the result is signed with the APK Signature Scheme v2 instead.
func Rebrand(base []byte, branding Branding, signer *Signer) ([]byte, error) {
	if signer == nil || signer.PrivateKey == nil || signer.Cert == nil {
		return nil, errors.New("apkbrand: signer is required")
	}
	if strings.TrimSpace(branding.Label) == "" {
		return nil, errors.New("apkbrand: label is required")
	}

	entries, err := readEntries(base)
	if err != nil {
		return nil, fmt.Errorf("apkbrand: read archive: %w", err)
	}

	resourcesFound := false
	for i := range entries {
		switch {
		case entries[i].Name == "resources.arsc":
			patched, err := setResourceString(entries[i].Data, "app_name", branding.Label)
			if err != nil {
				return nil, fmt.Errorf("apkbrand: patch resources: %w", err)
			}
			entries[i].Data = patched
			resourcesFound = true
		case isLauncherIcon(entries[i].Name):
			if len(branding.IconPNG) > 0 {
				entries[i].Data = branding.IconPNG
			}
		case entries[i].Name == "assets/www/logo.png":
			if len(branding.IconPNG) > 0 {
				entries[i].Data = branding.IconPNG
			}
		case entries[i].Name == "assets/www/index.html":
			entries[i].Data = []byte(titlePattern.ReplaceAllString(string(entries[i].Data), "<title>"+branding.Label+"</title>"))
		}
	}
	if !resourcesFound {
		return nil, errors.New("apkbrand: resources.arsc not found")
	}

	archive := buildArchive(entries)
	return signV2(archive, signer)
}

// isLauncherIcon reports whether the entry holds a launcher icon of the app.
func isLauncherIcon(name string) bool {
	if !strings.HasPrefix(name, "res/") {
		return false
	}
	base := filepath.Base(name)
	switch base {
	case "ic_launcher.png", "ic_launcher_round.png", "ic_launcher.webp":
		return true
	}
	return false
}

// GenerateSignerFile creates a new signing key at the given path. The file holds the
// private key and the certificate, it can be used by the server and by the build
// script of the worker APK.
func GenerateSignerFile(path string) (*Signer, error) {
	signer, err := GenerateSigner()
	if err != nil {
		return nil, err
	}
	if directory := filepath.Dir(path); directory != "" {
		if err := os.MkdirAll(directory, 0o755); err != nil {
			return nil, err
		}
	}
	if err := os.WriteFile(path, signerPEM(signer), 0o600); err != nil {
		return nil, err
	}
	return signer, nil
}

// SplitSignerPEM splits a signing key bundle into the private key and the
// certificate, both in PEM format. The Android build tools expect them separately.
func SplitSignerPEM(bundle []byte) (privateKey []byte, certificate []byte, err error) {
	rest := bundle
	for {
		var block *pem.Block
		block, rest = pem.Decode(rest)
		if block == nil {
			break
		}
		switch block.Type {
		case "PRIVATE KEY":
			privateKey = pem.EncodeToMemory(block)
		case "RSA PRIVATE KEY":
			privateKey = pem.EncodeToMemory(block)
		case "CERTIFICATE":
			certificate = pem.EncodeToMemory(block)
		}
	}
	if len(privateKey) == 0 || len(certificate) == 0 {
		return nil, nil, errors.New("apkbrand: the signing key bundle is incomplete")
	}
	return privateKey, certificate, nil
}

// LoadSigner reads a PEM encoded RSA private key and its self signed certificate from
// the given path. When the file does not exist a new key pair is generated and stored.
func LoadSigner(path string) (*Signer, error) {
	if path == "" {
		return nil, errors.New("apkbrand: signing key path is required")
	}
	raw, err := os.ReadFile(path)
	if err == nil && len(raw) > 0 {
		return parseSignerPEM(raw)
	}
	if !errors.Is(err, os.ErrNotExist) {
		return nil, err
	}

	signer, err := GenerateSigner()
	if err != nil {
		return nil, err
	}
	if dir := filepath.Dir(path); dir != "" {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			return nil, err
		}
	}
	if err := os.WriteFile(path, signerPEM(signer), 0o600); err != nil {
		return nil, err
	}
	return signer, nil
}

// GenerateSigner creates a fresh RSA key pair with a self signed certificate.
func GenerateSigner() (*Signer, error) {
	key, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		return nil, err
	}
	serial, err := rand.Int(rand.Reader, new(big.Int).Lsh(big.NewInt(1), 128))
	if err != nil {
		return nil, err
	}
	template := x509.Certificate{
		SerialNumber:          serial,
		Subject:               pkix.Name{CommonName: "BYOG Worker", Organization: []string{"BYOG"}},
		NotBefore:             time.Now().Add(-time.Hour),
		NotAfter:              time.Now().AddDate(10, 0, 0),
		KeyUsage:              x509.KeyUsageDigitalSignature | x509.KeyUsageKeyEncipherment,
		BasicConstraintsValid: true,
	}
	der, err := x509.CreateCertificate(rand.Reader, &template, &template, &key.PublicKey, key)
	if err != nil {
		return nil, err
	}
	cert, err := x509.ParseCertificate(der)
	if err != nil {
		return nil, err
	}
	return &Signer{PrivateKey: key, Cert: cert}, nil
}

// signerPEM serialises the key and the certificate of a signer.
//
// The key is written in the PKCS#1 format, the Android build tools accept it
// directly and the server reads it back without a conversion.
func signerPEM(signer *Signer) []byte {
	var out []byte
	out = append(out, pem.EncodeToMemory(&pem.Block{Type: "RSA PRIVATE KEY", Bytes: x509.MarshalPKCS1PrivateKey(signer.PrivateKey)})...)
	out = append(out, pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: signer.Cert.Raw})...)
	return out
}

// parseSignerPEM restores a signer from a PEM bundle.
func parseSignerPEM(raw []byte) (*Signer, error) {
	var key *rsa.PrivateKey
	var cert *x509.Certificate
	rest := raw
	for {
		var block *pem.Block
		block, rest = pem.Decode(rest)
		if block == nil {
			break
		}
		switch block.Type {
		case "PRIVATE KEY":
			parsed, err := x509.ParsePKCS8PrivateKey(block.Bytes)
			if err != nil {
				return nil, err
			}
			rsaKey, ok := parsed.(*rsa.PrivateKey)
			if !ok {
				return nil, errors.New("apkbrand: unexpected private key type")
			}
			key = rsaKey
		case "RSA PRIVATE KEY":
			parsed, err := x509.ParsePKCS1PrivateKey(block.Bytes)
			if err != nil {
				return nil, err
			}
			key = parsed
		case "CERTIFICATE":
			parsed, err := x509.ParseCertificate(block.Bytes)
			if err != nil {
				return nil, err
			}
			cert = parsed
		}
	}
	if key == nil || cert == nil {
		return nil, errors.New("apkbrand: incomplete signing key")
	}
	return &Signer{PrivateKey: key, Cert: cert}, nil
}

// signerCache keeps the loaded signing key in memory so that every download does not
// have to read it from disk.
var (
	signerCacheMu sync.Mutex
	signerCache   = map[string]*Signer{}
)

// CachedSigner returns the signer stored at path, loading it at most once per path.
func CachedSigner(path string) (*Signer, error) {
	signerCacheMu.Lock()
	defer signerCacheMu.Unlock()
	if signer, ok := signerCache[path]; ok {
		return signer, nil
	}
	signer, err := LoadSigner(path)
	if err != nil {
		return nil, err
	}
	signerCache[path] = signer
	return signer, nil
}
