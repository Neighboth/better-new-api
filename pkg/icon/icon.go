// Package icon writes the icon files of the system.
//
// The dashboard hands out the icon of the installation, the icon of the client
// of Windows is a file of the format of the icons of Windows.
package icon

import (
	"encoding/binary"
)

// FromPNG builds the file of an icon from a picture in the format of PNG.
//
// The format of the icons of Windows accepts a picture in the format of PNG, the
// picture is the content of the file.
func FromPNG(pngData []byte) []byte {
	if len(pngData) < 8 || string(pngData[1:4]) != "PNG" {
		return nil
	}

	// The measures of the picture are in the header of the file.
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
