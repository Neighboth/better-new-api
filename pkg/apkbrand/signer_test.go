package apkbrand

import (
	"bytes"
	"os"
	"path/filepath"
	"sync"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestRebrandUsesStableInstallationSigner(t *testing.T) {
	path := baseAPKPath()
	if path == "" {
		t.Skip("prebuilt APK not available")
	}
	raw, err := os.ReadFile(path)
	require.NoError(t, err)
	keyPath := filepath.Join(t.TempDir(), "private.pem")
	signer, err := LoadSigner(keyPath)
	require.NoError(t, err)
	icon, err := RenderIcon(testIconPNG(t), 192)
	require.NoError(t, err)
	first, err := Rebrand(raw, Branding{Label: testLabel, IconPNG: icon}, signer)
	require.NoError(t, err)
	nextSigner, err := LoadSigner(keyPath)
	require.NoError(t, err)
	second, err := Rebrand(raw, Branding{Label: "Updated label", IconPNG: icon}, nextSigner)
	require.NoError(t, err)
	firstChain, err := signerCertificates(first)
	require.NoError(t, err)
	secondChain, err := signerCertificates(second)
	require.NoError(t, err)
	require.NotEmpty(t, firstChain)
	assert.Equal(t, firstChain, secondChain, "updates for one installation must have the same signer")
	privateKey, certificate, err := SplitSignerPEM(mustRead(t, keyPath))
	require.NoError(t, err)
	assert.True(t, bytes.Contains(privateKey, []byte("PRIVATE KEY")))
	assert.True(t, bytes.Contains(certificate, []byte("CERTIFICATE")))
}

func mustRead(t *testing.T, path string) []byte {
	t.Helper()
	data, err := os.ReadFile(path)
	require.NoError(t, err)
	return data
}

func TestConcurrentSignerCreationUsesOneKey(t *testing.T) {
	path := filepath.Join(t.TempDir(), "keys", "private.pem")
	const workers = 5
	certs := make([][]byte, workers)
	errs := make([]error, workers)
	var wg sync.WaitGroup
	for i := range certs {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			signer, err := LoadSigner(path)
			errs[i] = err
			if err == nil {
				certs[i] = signer.Cert.Raw
			}
		}(i)
	}
	wg.Wait()
	for i := range certs {
		require.NoError(t, errs[i])
		require.Equal(t, certs[0], certs[i])
	}
	info, err := os.Stat(path)
	require.NoError(t, err)
	if os.PathSeparator != '\\' {
		require.Equal(t, os.FileMode(0600), info.Mode().Perm())
	}
}
