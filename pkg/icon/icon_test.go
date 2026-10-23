package icon

import (
	"bytes"
	"image"
	"image/color"
	"image/png"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
)

// TestPNGFromSiteIcon protects the icon of the installation: the default icon
// of the site is a file of the icons of Windows, the resource of the program
// needs a PNG picture that fits it.
func TestPNGFromSiteIcon(t *testing.T) {
	data, err := os.ReadFile(filepath.Join("..", "..", "web", "public", "favicon.ico"))
	require.NoError(t, err)
	require.True(t, isICO(data))

	pngData := PNG(data, MaxResourceSize)
	require.NotEmpty(t, pngData)
	require.LessOrEqual(t, len(pngData), MaxResourceSize)

	picture, err := png.Decode(bytes.NewReader(pngData))
	require.NoError(t, err)
	bounds := picture.Bounds()
	require.Equal(t, bounds.Dx(), bounds.Dy())
	require.LessOrEqual(t, bounds.Dx(), 256)
}

// TestFromImageKeepsSiteIcon protects the icon file of the client: a file of the
// icons of Windows is written as it is, the browser of the site chooses the
// picture that it needs.
func TestFromImageKeepsSiteIcon(t *testing.T) {
	data, err := os.ReadFile(filepath.Join("..", "..", "web", "public", "favicon.ico"))
	require.NoError(t, err)
	require.Equal(t, data, FromImage(data))
}

// TestFromPNGWritesTheIcon protects the icon of the program of the client: the
// picture of the installation is written into a file of the icons of Windows.
func TestFromPNGWritesTheIcon(t *testing.T) {
	picture := image.NewNRGBA(image.Rect(0, 0, 32, 32))
	picture.SetNRGBA(0, 0, color.NRGBA{R: 255, A: 255})
	var buffer bytes.Buffer
	require.NoError(t, png.Encode(&buffer, picture))

	iconData := FromPNG(buffer.Bytes())
	require.NotEmpty(t, iconData)
	require.Equal(t, byte(0), iconData[0])
	require.Equal(t, byte(1), iconData[2])
	require.Equal(t, uint32(buffer.Len()), uint32(len(iconData))-22)
	require.Equal(t, buffer.Bytes(), iconData[22:])

	require.Nil(t, FromPNG([]byte("PNG")))
}
