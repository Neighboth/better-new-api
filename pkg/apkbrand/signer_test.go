package apkbrand

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestRebrandKeepsTheSignerOfTheApk protects the update of an installed application.
//
// Android refuses to install an APK whose signer differs from the one of the
// installed application ("App not installed"). The server signs the rebranded APK
// with the key of the repository, therefore the signer of the result has to be the
// same as the signer of the APK that was built from the sources.
func TestRebrandKeepsTheSignerOfTheApk(t *testing.T) {
	path := baseAPKPath()
	if path == "" {
		t.Skip("prebuilt APK not available")
	}
	raw, err := os.ReadFile(path)
	require.NoError(t, err)

	keyPath := filepath.Join("..", "..", "worker", "mobile", "apk_signing_key.pem")
	signer, err := LoadSigner(keyPath)
	if err != nil {
		t.Skipf("the shared signing key is not available: %v", err)
	}

	baseChain, err := signerCertificates(raw)
	require.NoError(t, err)
	require.NotEmpty(t, baseChain)

	icon, err := RenderIcon(testIconPNG(t), 192)
	require.NoError(t, err)

	rebranded, err := Rebrand(raw, Branding{Label: testLabel, IconPNG: icon}, signer)
	require.NoError(t, err)

	rebrandedChain, err := signerCertificates(rebranded)
	require.NoError(t, err)
	require.NotEmpty(t, rebrandedChain)
	assert.Equal(t, baseChain, rebrandedChain, "the signer of the APK changed, an installed application could not be updated")
}

// TestSigningKeyBundleIsUsable makes sure that the key file of the repository can be
// read by the server and by the build script.
func TestSigningKeyBundleIsUsable(t *testing.T) {
	keyPath := filepath.Join("..", "..", "worker", "mobile", "apk_signing_key.pem")
	raw, err := os.ReadFile(keyPath)
	if err != nil {
		t.Skip("the shared signing key is not available")
	}

	signer, err := parseSignerPEM(raw)
	require.NoError(t, err)
	require.NotNil(t, signer.Cert)
	assert.NotEmpty(t, signer.Cert.Raw)

	privateKey, certificate, err := SplitSignerPEM(raw)
	require.NoError(t, err)
	assert.True(t, bytes.Contains(privateKey, []byte("PRIVATE KEY")))
	assert.True(t, bytes.Contains(certificate, []byte("CERTIFICATE")))
}
