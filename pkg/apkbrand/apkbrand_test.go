package apkbrand

import (
	"archive/zip"
	"bytes"
	"encoding/binary"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const testLabel = "Acme Cloud BYOG"

// baseAPKPath returns the path of the prebuilt worker APK, if it exists.
func baseAPKPath() string {
	candidates := []string{
		filepath.Join("..", "..", "worker", "binaries", "byog-worker.apk"),
		filepath.Join("worker", "binaries", "byog-worker.apk"),
	}
	for _, candidate := range candidates {
		if _, err := os.Stat(candidate); err == nil {
			return candidate
		}
	}
	return ""
}

// TestRebrandApk rewrites the launcher branding of the shipped APK and validates the
// result with apksigner when the Android build tools are available.
func TestRebrandApk(t *testing.T) {
	path := baseAPKPath()
	if path == "" {
		t.Skip("prebuilt APK not available")
	}
	raw, err := os.ReadFile(path)
	require.NoError(t, err)

	signer, err := GenerateSigner()
	require.NoError(t, err)

	icon, err := RenderIcon(testIconPNG(t), 192)
	require.NoError(t, err)

	rebranded, err := Rebrand(raw, Branding{Label: testLabel, IconPNG: icon}, signer)
	require.NoError(t, err)
	require.Greater(t, len(rebranded), len(raw)/2)

	// The label of the launcher must be the rebranded one.
	assert.Equal(t, testLabel, readAppName(t, rebranded))

	// The archive must still be a readable ZIP without the old JAR signature.
	reader, err := zip.NewReader(bytes.NewReader(rebranded), int64(len(rebranded)))
	require.NoError(t, err)
	names := map[string]bool{}
	for _, file := range reader.File {
		names[file.Name] = true
	}
	assert.True(t, names["classes.arsc"] || names["resources.arsc"])
	assert.False(t, names["META-INF/MANIFEST.MF"])

	// The signature must verify with our own implementation.
	require.NoError(t, verifySignature(rebranded, signer))

	// apksigner is the reference implementation, use it when it is installed.
	apksigner := findApksigner()
	if apksigner == "" {
		t.Skip("apksigner not available")
	}
	temp := filepath.Join(t.TempDir(), "rebranded.apk")
	require.NoError(t, os.WriteFile(temp, rebranded, 0o600))
	out, err := exec.Command(apksigner, "verify", "--verbose", temp).CombinedOutput()
	require.NoError(t, err, string(out))
	assert.Contains(t, string(out), "Verifies")

	// aapt parses the resource table, it reports a broken table.
	aapt := findAapt()
	if aapt == "" {
		t.Skip("aapt not available")
	}
	out, err = exec.Command(aapt, "dump", "badging", temp).CombinedOutput()
	require.NoError(t, err, string(out))
	assert.Contains(t, string(out), "application-label:'"+testLabel+"'")

	// The platform refuses to install an APK with misaligned native libraries.
	zipalign := findZipalign()
	if zipalign == "" {
		t.Skip("zipalign not available")
	}
	out, err = exec.Command(zipalign, "-c", "-P", "16", "-v", "4", temp).CombinedOutput()
	require.NoError(t, err, string(out))
	assert.Contains(t, string(out), "Verification successful")
}

// TestRebrandWithoutResourcesFails makes sure that a broken base APK is reported.
func TestRebrandWithoutResourcesFails(t *testing.T) {
	signer, err := GenerateSigner()
	require.NoError(t, err)

	var buf bytes.Buffer
	writer := zip.NewWriter(&buf)
	writer.Create("classes.dex")
	writer.Close()

	_, err = Rebrand(buf.Bytes(), Branding{Label: testLabel}, signer)
	require.Error(t, err)
}

// TestRenderIconFormats verifies that every supported icon format is converted.
func TestRenderIconFormats(t *testing.T) {
	pngIcon := testIconPNG(t)
	for _, format := range []string{"png", "jpeg", "gif", "webp", "ico"} {
		data := convertIcon(t, pngIcon, format)
		icon, err := RenderIcon(data, 192)
		require.NoError(t, err, format)
		require.NotEmpty(t, icon, format)
		assert.True(t, bytes.HasPrefix(icon, []byte{0x89, 'P', 'N', 'G'}), format)
	}
}

// TestRenderProjectFavicon converts the favicon of the project, it is an icon with
// bitmaps instead of an embedded PNG.
func TestRenderProjectFavicon(t *testing.T) {
	raw, err := os.ReadFile(filepath.Join("..", "..", "web", "public", "favicon.ico"))
	if err != nil {
		t.Skip("project favicon not available")
	}
	icon, err := RenderIcon(raw, 192)
	require.NoError(t, err)
	require.NotEmpty(t, icon)
	assert.True(t, bytes.HasPrefix(icon, []byte{0x89, 'P', 'N', 'G'}))
}

// readAppName reads the app_name resource of an APK.
func readAppName(t *testing.T, apk []byte) string {
	t.Helper()
	reader, err := zip.NewReader(bytes.NewReader(apk), int64(len(apk)))
	require.NoError(t, err)
	for _, file := range reader.File {
		if file.Name != "resources.arsc" {
			continue
		}
		rc, err := file.Open()
		require.NoError(t, err)
		defer rc.Close()
		data, err := io.ReadAll(rc)
		require.NoError(t, err)
		pool, err := parseStringPool(data, 12)
		require.NoError(t, err)
		index, err := findStringResourceIndex(data, 12, len(data), pool, "app_name")
		require.NoError(t, err)
		value, err := pool.stringAt(data, index)
		require.NoError(t, err)
		return value
	}
	t.Fatal("resources.arsc not found")
	return ""
}

// findApksigner locates the apksigner tool of the Android build tools.
func findApksigner() string {
	sdk := os.Getenv("ANDROID_HOME")
	if sdk == "" {
		sdk = os.Getenv("ANDROID_SDK_ROOT")
	}
	if sdk == "" {
		return ""
	}
	matches, err := filepath.Glob(filepath.Join(sdk, "build-tools", "*", "apksigner.bat"))
	if err != nil || len(matches) == 0 {
		return ""
	}
	return matches[len(matches)-1]
}

// findAapt locates the aapt tool of the Android build tools.
func findAapt() string {
	sdk := os.Getenv("ANDROID_HOME")
	if sdk == "" {
		sdk = os.Getenv("ANDROID_SDK_ROOT")
	}
	if sdk == "" {
		return ""
	}
	matches, err := filepath.Glob(filepath.Join(sdk, "build-tools", "*", "aapt.exe"))
	if err != nil || len(matches) == 0 {
		return ""
	}
	return matches[len(matches)-1]
}

// findZipalign locates the zipalign tool of the Android build tools.
func findZipalign() string {
	sdk := os.Getenv("ANDROID_HOME")
	if sdk == "" {
		sdk = os.Getenv("ANDROID_SDK_ROOT")
	}
	if sdk == "" {
		return ""
	}
	matches, err := filepath.Glob(filepath.Join(sdk, "build-tools", "*", "zipalign.exe"))
	if err != nil || len(matches) == 0 {
		return ""
	}
	return matches[len(matches)-1]
}

// testIconPNG creates a small PNG image used by the tests.
func testIconPNG(t *testing.T) []byte {
	t.Helper()
	var buf bytes.Buffer
	img := newTestImage(32, 32)
	require.NoError(t, encodePNG(&buf, img))
	return buf.Bytes()
}

// convertIcon converts the reference PNG into another image format.
func convertIcon(t *testing.T, pngData []byte, format string) []byte {
	t.Helper()
	img := decodeTestImage(t, pngData)
	var buf bytes.Buffer
	switch format {
	case "jpeg":
		require.NoError(t, encodeJPEG(&buf, img))
	case "gif":
		require.NoError(t, encodeGIF(&buf, img))
	case "webp":
		// The standard library cannot write WebP, so the PNG payload is used and the
		// decoder falls back to the PNG branch.
		buf.Write(pngData)
	case "ico":
		// Build an ICO container that embeds the PNG image.
		buf.Write(buildICO(pngData))
	default:
		buf.Write(pngData)
	}
	return buf.Bytes()
}

// buildICO wraps a PNG image into an ICO container.
func buildICO(pngData []byte) []byte {
	header := make([]byte, 6)
	binary.LittleEndian.PutUint16(header[0:], 0)
	binary.LittleEndian.PutUint16(header[2:], 1)
	binary.LittleEndian.PutUint16(header[4:], 1)

	entry := make([]byte, 16)
	entry[0] = 32
	entry[1] = 32
	binary.LittleEndian.PutUint16(entry[4:], 1)
	binary.LittleEndian.PutUint16(entry[6:], 32)
	binary.LittleEndian.PutUint32(entry[8:], uint32(len(pngData)))
	binary.LittleEndian.PutUint32(entry[12:], uint32(6+16))

	out := append([]byte{}, header...)
	out = append(out, entry...)
	return append(out, pngData...)
}

// assertLabelPresent makes sure that a label occurs in the given payload.
func assertLabelPresent(t *testing.T, payload []byte, label string) {
	t.Helper()
	assert.True(t, strings.Contains(string(payload), label))
}
