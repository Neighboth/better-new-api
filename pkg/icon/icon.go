// Package icon writes the icon files of the system.
//
// The dashboard hands out the icon of the installation, the icon of the client
// of Windows is a file of the format of the icons of Windows.
package icon

import (
	"bytes"
	"encoding/binary"
	"image"
	"image/color"
	stddraw "image/draw"
	"image/png"

	_ "image/gif"
	_ "image/jpeg"

	xdraw "golang.org/x/image/draw"
	_ "golang.org/x/image/webp"
)

// MaxResourceSize is the size of the picture that the resource of the icons of
// the program of the client holds.
const MaxResourceSize = 14026

// FromPNG builds the file of an icon from a picture in the format of PNG.
//
// The format of the icons of Windows accepts a picture in the format of PNG, the
// picture is the content of the file.
func FromPNG(pngData []byte) []byte {
	if len(pngData) < 24 || string(pngData[1:4]) != "PNG" {
		return nil
	}

	width, height := binary.BigEndian.Uint32(pngData[16:20]), binary.BigEndian.Uint32(pngData[20:24])

	file := make([]byte, 0, 22+len(pngData))

	header := make([]byte, 6)
	binary.LittleEndian.PutUint16(header[0:], 0)
	binary.LittleEndian.PutUint16(header[2:], 1)
	binary.LittleEndian.PutUint16(header[4:], 1)
	file = append(file, header...)

	entry := make([]byte, 16)
	if width < 256 {
		entry[0] = byte(width)
	}
	if height < 256 {
		entry[1] = byte(height)
	}
	binary.LittleEndian.PutUint16(entry[4:], 1)
	binary.LittleEndian.PutUint16(entry[6:], 32)
	binary.LittleEndian.PutUint32(entry[8:], uint32(len(pngData)))
	binary.LittleEndian.PutUint32(entry[12:], 22)
	file = append(file, entry...)

	return append(file, pngData...)
}

// FromImage builds the file of an icon from a picture of the installation.
//
// A file of the icons of Windows is used as it is, a picture in the format of
// PNG is written into the file, another picture is read and written as PNG.
func FromImage(data []byte) []byte {
	if isICO(data) {
		return data
	}
	pngData := PNG(data, 256*1024)
	if len(pngData) == 0 {
		return nil
	}
	return FromPNG(pngData)
}

// PNG reads the picture of the installation and writes it as a PNG picture for
// the resource of the icons of the program.
//
// The picture is the icon of the installation, the file of the icon of the site
// is a file of the icons of Windows or a PNG picture. The largest picture of the
// file is used, a picture that does not fit the resource of the program is
// written smaller. The function returns nil when the picture can not be read.
func PNG(data []byte, maxBytes int) []byte {
	if maxBytes <= 0 {
		maxBytes = MaxResourceSize
	}
	if isPNG(data) {
		if side := pngSide(data); side > 0 && side <= 256 && len(data) <= maxBytes {
			return data
		}
		if picture, err := png.Decode(bytes.NewReader(data)); err == nil {
			return fitPNG(picture, maxBytes)
		}
		return nil
	}
	if isICO(data) {
		if picture := readICO(data); picture != nil {
			return fitPNG(picture, maxBytes)
		}
		return nil
	}
	if picture, _, err := image.Decode(bytes.NewReader(data)); err == nil {
		return fitPNG(picture, maxBytes)
	}
	return nil
}

func isPNG(data []byte) bool {
	return len(data) >= 8 && data[0] == 0x89 && string(data[1:4]) == "PNG"
}

func isICO(data []byte) bool {
	return len(data) >= 6 && data[0] == 0 && data[1] == 0 && data[2] == 1 && data[3] == 0
}

func pngSide(data []byte) int {
	if len(data) < 24 {
		return 0
	}
	width := int(binary.BigEndian.Uint32(data[16:20]))
	height := int(binary.BigEndian.Uint32(data[20:24]))
	if width > height {
		return width
	}
	return height
}

// fitPNG writes the picture into a square PNG picture that fits the resource of
// the program.
//
// The picture of a site is not square and it can be large, the picture is placed
// in the middle of a square and it is written smaller until it fits. A picture
// that is too small for the resource is written at the size of the standard
// icon.
func fitPNG(picture image.Image, maxBytes int) []byte {
	bounds := picture.Bounds()
	width, height := bounds.Dx(), bounds.Dy()
	if width <= 0 || height <= 0 || width > 4096 || height > 4096 {
		return nil
	}

	side := width
	if height > side {
		side = height
	}
	canvas := image.NewNRGBA(image.Rect(0, 0, side, side))
	offsetX := (side - width) / 2
	offsetY := (side - height) / 2
	stddraw.Draw(canvas, image.Rect(offsetX, offsetY, offsetX+width, offsetY+height), picture, bounds.Min, stddraw.Src)

	target := side
	if target > 256 {
		target = 256
	}
	if target < 16 {
		target = 16
	}
	for target >= 16 {
		scaled := canvas
		if target != side {
			scaled = image.NewNRGBA(image.Rect(0, 0, target, target))
			xdraw.CatmullRom.Scale(scaled, scaled.Bounds(), canvas, canvas.Bounds(), xdraw.Over, nil)
		}
		var buffer bytes.Buffer
		encoder := png.Encoder{CompressionLevel: png.BestCompression}
		if err := encoder.Encode(&buffer, scaled); err == nil {
			if buffer.Len() <= maxBytes || target == 16 {
				return buffer.Bytes()
			}
		}
		target /= 2
	}
	return nil
}

// The picture of the icons of Windows.
//
// The file holds one or more pictures, every picture is a PNG picture or a
// bitmap of the system. The pictures are read from the largest to the smallest,
// the first picture that is read is the icon of the installation.
func readICO(data []byte) image.Image {
	if !isICO(data) {
		return nil
	}
	count := int(binary.LittleEndian.Uint16(data[4:6]))
	if count <= 0 || len(data) < 6+count*16 {
		return nil
	}

	type candidate struct {
		area   int
		offset int
		size   int
		width  int
		height int
	}
	candidates := make([]candidate, 0, count)
	for index := 0; index < count; index++ {
		entry := data[6+index*16 : 6+(index+1)*16]
		width := int(entry[0])
		height := int(entry[1])
		if width == 0 {
			width = 256
		}
		if height == 0 {
			height = 256
		}
		size := int(binary.LittleEndian.Uint32(entry[8:12]))
		offset := int(binary.LittleEndian.Uint32(entry[12:16]))
		if size <= 0 || offset <= 0 || offset+size > len(data) {
			continue
		}
		candidates = append(candidates, candidate{
			area:   width * height,
			offset: offset,
			size:   size,
			width:  width,
			height: height,
		})
	}

	for len(candidates) > 0 {
		best := 0
		for index := 1; index < len(candidates); index++ {
			if candidates[index].area > candidates[best].area {
				best = index
			}
		}
		entry := candidates[best]
		candidates = append(candidates[:best], candidates[best+1:]...)

		picture := data[entry.offset : entry.offset+entry.size]
		if isPNG(picture) {
			if decoded, err := png.Decode(bytes.NewReader(picture)); err == nil {
				return decoded
			}
			continue
		}
		if decoded := readICOBitmap(picture, entry.width, entry.height); decoded != nil {
			return decoded
		}
	}
	return nil
}

// readICOBitmap reads a bitmap of the system of the icons of Windows.
func readICOBitmap(data []byte, width int, height int) image.Image {
	if len(data) < 40 {
		return nil
	}
	headerSize := int(binary.LittleEndian.Uint32(data[0:4]))
	if headerSize < 40 || headerSize > len(data) {
		return nil
	}
	bitmapWidth := int(int32(binary.LittleEndian.Uint32(data[4:8])))
	bitmapHeight := int(int32(binary.LittleEndian.Uint32(data[8:12])))
	bitCount := int(binary.LittleEndian.Uint16(data[14:16]))
	compression := binary.LittleEndian.Uint32(data[16:20])
	if bitmapWidth <= 0 || bitmapHeight <= 0 || compression != 0 {
		return nil
	}
	if bitmapHeight > 0 {
		bitmapHeight /= 2
	} else {
		bitmapHeight = -bitmapHeight
	}
	if width <= 0 {
		width = bitmapWidth
	}
	if height <= 0 {
		height = bitmapHeight
	}
	if width <= 0 || height <= 0 || width > 1024 || height > 1024 {
		return nil
	}
	if bitCount != 1 && bitCount != 4 && bitCount != 8 && bitCount != 24 && bitCount != 32 {
		return nil
	}

	paletteBytes := 0
	if bitCount <= 8 {
		colors := int(binary.LittleEndian.Uint32(data[32:36]))
		if colors == 0 {
			colors = 1 << bitCount
		}
		paletteBytes = colors * 4
	}
	pixels := headerSize + paletteBytes
	rowSize := ((width*bitCount + 31) / 32) * 4
	if pixels+rowSize*height > len(data) {
		return nil
	}

	picture := image.NewNRGBA(image.Rect(0, 0, width, height))
	alphaUsed := false
	for y := 0; y < height; y++ {
		row := data[pixels+(height-1-y)*rowSize:]
		for x := 0; x < width; x++ {
			var red, green, blue, alpha byte
			switch bitCount {
			case 32:
				pixel := row[x*4:]
				blue, green, red, alpha = pixel[0], pixel[1], pixel[2], pixel[3]
				if alpha != 0 {
					alphaUsed = true
				}
			case 24:
				pixel := row[x*3:]
				blue, green, red, alpha = pixel[0], pixel[1], pixel[2], 255
			case 8:
				red, green, blue, alpha = icoColor(data[headerSize:], int(row[x]), 255)
			case 4:
				value := row[x/2]
				if x%2 == 0 {
					red, green, blue, alpha = icoColor(data[headerSize:], int(value>>4), 255)
				} else {
					red, green, blue, alpha = icoColor(data[headerSize:], int(value&0x0F), 255)
				}
			case 1:
				value := row[x/8]
				red, green, blue, alpha = icoColor(data[headerSize:], int((value>>uint(7-x%8))&1), 255)
			}
			picture.SetNRGBA(x, y, color.NRGBA{R: red, G: green, B: blue, A: alpha})
		}
	}

	if !alphaUsed && bitCount == 32 {
		// The icons of old programs write the alpha of every pixel as zero,
		// the mask of the icon holds the transparency of the picture.
		for index := 0; index < len(picture.Pix); index += 4 {
			picture.Pix[index+3] = 255
		}
	}
	return picture
}

func icoColor(palette []byte, index int, alpha byte) (byte, byte, byte, byte) {
	offset := index * 4
	if offset < 0 || offset+3 >= len(palette) {
		return 0, 0, 0, alpha
	}
	return palette[offset+2], palette[offset+1], palette[offset], alpha
}
