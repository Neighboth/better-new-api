package apkbrand

import (
	"bytes"
	"image"
	"image/color"
	"image/gif"
	"image/jpeg"
	"image/png"
	"testing"

	"github.com/stretchr/testify/require"
)

// newTestImage creates a small square test image.
func newTestImage(width, height int) image.Image {
	img := image.NewRGBA(image.Rect(0, 0, width, height))
	for y := 0; y < height; y++ {
		for x := 0; x < width; x++ {
			img.Set(x, y, color.RGBA{R: uint8(x * 8), G: uint8(y * 8), B: 128, A: 255})
		}
	}
	return img
}

// encodePNG encodes an image as PNG.
func encodePNG(buf *bytes.Buffer, img image.Image) error {
	return png.Encode(buf, img)
}

// encodeJPEG encodes an image as JPEG.
func encodeJPEG(buf *bytes.Buffer, img image.Image) error {
	return jpeg.Encode(buf, img, nil)
}

// encodeGIF encodes an image as GIF.
func encodeGIF(buf *bytes.Buffer, img image.Image) error {
	return gif.Encode(buf, img, nil)
}

// decodeTestImage decodes a PNG image.
func decodeTestImage(t *testing.T, data []byte) image.Image {
	t.Helper()
	img, err := png.Decode(bytes.NewReader(data))
	require.NoError(t, err)
	return img
}
