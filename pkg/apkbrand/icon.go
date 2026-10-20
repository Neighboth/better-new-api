package apkbrand

import (
	"bytes"
	"encoding/binary"
	"errors"
	"image"
	"image/color"
	"image/draw"
	"image/gif"
	"image/jpeg"
	"image/png"

	xdraw "golang.org/x/image/draw"
	"golang.org/x/image/webp"
)

// RenderIcon converts arbitrary image bytes into a square PNG icon.
//
// PNG, JPEG, GIF, WebP and ICO (both the PNG and the bitmap flavour) are supported.
// The image is scaled into a square canvas so that the launcher icon is not distorted.
func RenderIcon(data []byte, size int) ([]byte, error) {
	if len(data) == 0 {
		return nil, errors.New("apkbrand: empty icon")
	}
	if size <= 0 {
		size = 192
	}

	source, err := decodeImage(data)
	if err != nil {
		return nil, err
	}

	canvas := image.NewRGBA(image.Rect(0, 0, size, size))
	draw.Draw(canvas, canvas.Bounds(), image.NewUniform(color.Transparent), image.Point{}, draw.Src)

	bounds := source.Bounds()
	targetWidth, targetHeight := size, size
	if bounds.Dx() > 0 && bounds.Dy() > 0 {
		ratio := float64(size) / float64(max(bounds.Dx(), bounds.Dy()))
		targetWidth = int(float64(bounds.Dx())*ratio + 0.5)
		targetHeight = int(float64(bounds.Dy())*ratio + 0.5)
		if targetWidth < 1 {
			targetWidth = 1
		}
		if targetHeight < 1 {
			targetHeight = 1
		}
	}
	offsetX := (size - targetWidth) / 2
	offsetY := (size - targetHeight) / 2

	scaled := image.NewRGBA(image.Rect(0, 0, targetWidth, targetHeight))
	xdraw.CatmullRom.Scale(scaled, scaled.Bounds(), source, bounds, xdraw.Src, nil)
	draw.Draw(canvas, image.Rect(offsetX, offsetY, offsetX+targetWidth, offsetY+targetHeight), scaled, image.Point{}, draw.Over)

	var out bytes.Buffer
	if err := png.Encode(&out, canvas); err != nil {
		return nil, err
	}
	return out.Bytes(), nil
}

// decodeImage decodes the supported image formats.
func decodeImage(data []byte) (image.Image, error) {
	if img, err := png.Decode(bytes.NewReader(data)); err == nil {
		return img, nil
	}
	if img, err := jpeg.Decode(bytes.NewReader(data)); err == nil {
		return img, nil
	}
	if img, err := gif.Decode(bytes.NewReader(data)); err == nil {
		return img, nil
	}
	if img, err := webp.Decode(bytes.NewReader(data)); err == nil {
		return img, nil
	}
	if img, err := decodeICO(data); err == nil {
		return img, nil
	}
	return nil, errors.New("apkbrand: unsupported icon format")
}

// decodeICO decodes the largest image of an ICO container.
func decodeICO(data []byte) (image.Image, error) {
	if len(data) < 6 {
		return nil, errors.New("apkbrand: invalid ico")
	}
	count := int(binary.LittleEndian.Uint16(data[4:]))
	if count == 0 || len(data) < 6+count*16 {
		return nil, errors.New("apkbrand: invalid ico")
	}

	bestOffset := 0
	bestSize := 0
	bestArea := -1
	for i := 0; i < count; i++ {
		entry := data[6+i*16:]
		width := int(entry[0])
		height := int(entry[1])
		if width == 0 {
			width = 256
		}
		if height == 0 {
			height = 256
		}
		size := int(binary.LittleEndian.Uint32(entry[8:]))
		offset := int(binary.LittleEndian.Uint32(entry[12:]))
		if offset < 0 || size <= 0 || offset+size > len(data) {
			continue
		}
		if area := width * height; area > bestArea {
			bestArea = area
			bestOffset = offset
			bestSize = size
		}
	}
	if bestSize == 0 {
		return nil, errors.New("apkbrand: invalid ico")
	}

	payload := data[bestOffset : bestOffset+bestSize]
	if img, err := png.Decode(bytes.NewReader(payload)); err == nil {
		return img, nil
	}
	return decodeICOBitmap(payload)
}

// decodeICOBitmap decodes the bitmap flavour of an ICO image.
//
// The bitmap of an icon stores the colour data bottom up and doubles the height for
// the transparency mask. The alpha channel of the colour data is used when it carries
// information, otherwise the transparency mask defines the visible pixels.
func decodeICOBitmap(payload []byte) (image.Image, error) {
	if len(payload) < 40 {
		return nil, errors.New("apkbrand: invalid ico bitmap")
	}
	headerSize := int(binary.LittleEndian.Uint32(payload[0:]))
	width := int(int32(binary.LittleEndian.Uint32(payload[4:])))
	height := int(int32(binary.LittleEndian.Uint32(payload[8:])))
	bitCount := int(binary.LittleEndian.Uint16(payload[14:]))
	if headerSize < 40 || width <= 0 || height <= 0 {
		return nil, errors.New("apkbrand: invalid ico bitmap")
	}
	if height%2 != 0 {
		return nil, errors.New("apkbrand: invalid ico bitmap")
	}
	colorHeight := height / 2
	if bitCount != 32 {
		return nil, errors.New("apkbrand: unsupported ico bitmap")
	}

	rowBytes := width * 4
	canvas := image.NewNRGBA(image.Rect(0, 0, width, colorHeight))
	hasAlpha := false
	for row := 0; row < colorHeight; row++ {
		sourceRow := colorHeight - 1 - row
		offset := headerSize + sourceRow*rowBytes
		if offset+rowBytes > len(payload) {
			return nil, errors.New("apkbrand: truncated ico bitmap")
		}
		for x := 0; x < width; x++ {
			pixel := payload[offset+x*4:]
			canvas.SetNRGBA(x, row, color.NRGBA{R: pixel[2], G: pixel[1], B: pixel[0], A: pixel[3]})
			if pixel[3] != 0 {
				hasAlpha = true
			}
		}
	}
	if hasAlpha {
		return canvas, nil
	}

	// The transparency mask holds one bit per pixel, a set bit marks a transparent
	// pixel. The rows are stored bottom up and padded to four byte boundaries.
	maskOffset := headerSize + colorHeight*rowBytes
	maskStride := ((width + 31) / 32) * 4
	for row := 0; row < colorHeight; row++ {
		sourceRow := colorHeight - 1 - row
		offset := maskOffset + sourceRow*maskStride
		if offset+maskStride > len(payload) {
			break
		}
		for x := 0; x < width; x++ {
			if payload[offset+x/8]&(1<<(7-uint(x%8))) != 0 {
				canvas.SetNRGBA(x, row, color.NRGBA{})
			} else {
				canvas.SetNRGBA(x, row, color.NRGBA{R: canvas.NRGBAAt(x, row).R, G: canvas.NRGBAAt(x, row).G, B: canvas.NRGBAAt(x, row).B, A: 255})
			}
		}
	}
	return canvas, nil
}
