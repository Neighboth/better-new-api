package embedcfg

import (
	"encoding/binary"
	"fmt"
)

// PatchIcon replaces the icon resource in a Windows PE file.
//
// The icon of the installation is embedded into the program of the client, the
// program carries the name and the icon of the installation. The client does
// not ask the user for them.
//
// The icon data is a PNG picture. The picture is written into the resource of
// the program in place of the picture that was there, the program does not grow
// with it.
func PatchIcon(program []byte, iconData []byte) ([]byte, error) {
	if len(program) < 0x40 {
		return nil, fmt.Errorf("the program is too small")
	}

	// The offset of the PE header.
	peOffset := int(binary.LittleEndian.Uint32(program[0x3C:]))
	if peOffset+24 > len(program) {
		return nil, fmt.Errorf("the PE header is not found")
	}

	numSections := int(binary.LittleEndian.Uint16(program[peOffset+6:]))
	optHeaderSize := int(binary.LittleEndian.Uint16(program[peOffset+20:]))
	sectionTableOffset := peOffset + 24 + int(optHeaderSize)

	// Find the .rsrc section.
	var rsrcRA, rsrcRS, rsrcVA int
	var rsrcSectionIndex int
	found := false
	for i := 0; i < numSections; i++ {
		off := sectionTableOffset + i*40
		name := string(program[off : off+8])
		for len(name) > 0 && name[len(name)-1] == 0 {
			name = name[:len(name)-1]
		}
		if name == ".rsrc" {
			rsrcVA = int(binary.LittleEndian.Uint32(program[off+12:]))
			rsrcVS := int(binary.LittleEndian.Uint32(program[off+8:]))
			rsrcRA = int(binary.LittleEndian.Uint32(program[off+20:]))
			rsrcRS = int(binary.LittleEndian.Uint32(program[off+16:]))
			rsrcSectionIndex = i
			found = true
			_ = rsrcVS
			break
		}
	}
	if !found {
		return nil, fmt.Errorf("the resource section is not found")
	}

	// Parse the root resource directory.
	numNamed := int(binary.LittleEndian.Uint16(program[rsrcRA+12:]))
	numID := int(binary.LittleEndian.Uint16(program[rsrcRA+14:]))
	off := rsrcRA + 16

	var iconDataOffset, iconDataSize int
	var groupIconOffset, groupIconSize int

	for i := 0; i < numNamed+numID; i++ {
		typeID := binary.LittleEndian.Uint32(program[off:])
		nameOffset := binary.LittleEndian.Uint32(program[off+4:])
		isDir := (nameOffset & 0x80000000) != 0
		entryOffset := int(nameOffset & 0x7FFFFFFF)
		if typeID == 3 && isDir { // RT_ICON
			subDir := rsrcRA + entryOffset
			sn := int(binary.LittleEndian.Uint16(program[subDir+12:]))
			si := int(binary.LittleEndian.Uint16(program[subDir+14:]))
			soff := subDir + 16
			for j := 0; j < sn+si; j++ {
				soffset := binary.LittleEndian.Uint32(program[soff+4:])
				sisDir := (soffset & 0x80000000) != 0
				sentryOffset := int(soffset & 0x7FFFFFFF)
				if sisDir {
					langDir := rsrcRA + sentryOffset
					ln := int(binary.LittleEndian.Uint16(program[langDir+12:]))
					li := int(binary.LittleEndian.Uint16(program[langDir+14:]))
					loff := langDir + 16
					for k := 0; k < ln+li; k++ {
						dataOffset := binary.LittleEndian.Uint32(program[loff+4:])
						isData := (dataOffset & 0x80000000) == 0
						dataEntryOffset := int(dataOffset & 0x7FFFFFFF)
						if isData {
							deOff := rsrcRA + dataEntryOffset
							dataRVA := binary.LittleEndian.Uint32(program[deOff:])
							dataSize := binary.LittleEndian.Uint32(program[deOff+4:])
							rawOff := int(dataRVA) - rsrcVA + rsrcRA
							iconDataOffset = rawOff
							iconDataSize = int(dataSize)
						}
						loff += 8
					}
				}
				soff += 8
			}
		}
		if typeID == 14 && isDir { // RT_GROUP_ICON
			subDir := rsrcRA + entryOffset
			sn := int(binary.LittleEndian.Uint16(program[subDir+12:]))
			si := int(binary.LittleEndian.Uint16(program[subDir+14:]))
			soff := subDir + 16
			for j := 0; j < sn+si; j++ {
				soffset := binary.LittleEndian.Uint32(program[soff+4:])
				sisDir := (soffset & 0x80000000) != 0
				sentryOffset := int(soffset & 0x7FFFFFFF)
				if sisDir {
					langDir := rsrcRA + sentryOffset
					ln := int(binary.LittleEndian.Uint16(program[langDir+12:]))
					li := int(binary.LittleEndian.Uint16(program[langDir+14:]))
					loff := langDir + 16
					for k := 0; k < ln+li; k++ {
						dataOffset := binary.LittleEndian.Uint32(program[loff+4:])
						isData := (dataOffset & 0x80000000) == 0
						dataEntryOffset := int(dataOffset & 0x7FFFFFFF)
						if isData {
							deOff := rsrcRA + dataEntryOffset
							dataRVA := binary.LittleEndian.Uint32(program[deOff:])
							dataSize := binary.LittleEndian.Uint32(program[deOff+4:])
							rawOff := int(dataRVA) - rsrcVA + rsrcRA
							groupIconOffset = rawOff
							groupIconSize = int(dataSize)
						}
						loff += 8
					}
				}
				soff += 8
			}
		}
		off += 8
	}

	if iconDataOffset == 0 || iconDataSize == 0 {
		return nil, fmt.Errorf("the icon resource is not found")
	}
	if groupIconOffset == 0 || groupIconSize == 0 {
		return nil, fmt.Errorf("the group icon resource is not found")
	}
	if len(iconData) > iconDataSize {
		return nil, fmt.Errorf("the icon is too large: %d > %d", len(iconData), iconDataSize)
	}

	// Make a copy of the program.
	patched := make([]byte, len(program))
	copy(patched, program)

	// Replace the icon data (pad with zeros).
	for i := 0; i < iconDataSize; i++ {
		if i < len(iconData) {
			patched[iconDataOffset+i] = iconData[i]
		} else {
			patched[iconDataOffset+i] = 0
		}
	}

	// Update the RT_GROUP_ICON's fields.
	// The group icon data: GRPICONDIR (6 bytes) + GRPICONDIRENTRY (14 bytes).
	// Width is at offset 6, Height at offset 7, BytesInRes at offset 16.
	if len(iconData) >= 24 && string(iconData[1:4]) == "PNG" {
		width := binary.BigEndian.Uint32(iconData[16:20])
		height := binary.BigEndian.Uint32(iconData[20:24])
		if width <= 255 && height <= 255 {
			patched[groupIconOffset+6] = byte(width)
			patched[groupIconOffset+7] = byte(height)
		}
	}
	binary.LittleEndian.PutUint32(patched[groupIconOffset+16:], uint32(len(iconData)))

	// Update the RT_ICON data entry's size.
	// Find the data entry for RT_ICON and update its size.
	// We already have iconDataOffset, but we need the data entry offset.
	// Let's find it again and update the size.
	off = rsrcRA + 16
	for i := 0; i < numNamed+numID; i++ {
		typeID := binary.LittleEndian.Uint32(patched[off:])
		nameOffset := binary.LittleEndian.Uint32(patched[off+4:])
		isDir := (nameOffset & 0x80000000) != 0
		entryOffset := int(nameOffset & 0x7FFFFFFF)
		if typeID == 3 && isDir {
			subDir := rsrcRA + entryOffset
			sn := int(binary.LittleEndian.Uint16(patched[subDir+12:]))
			si := int(binary.LittleEndian.Uint16(patched[subDir+14:]))
			soff := subDir + 16
			for j := 0; j < sn+si; j++ {
				soffset := binary.LittleEndian.Uint32(patched[soff+4:])
				sisDir := (soffset & 0x80000000) != 0
				sentryOffset := int(soffset & 0x7FFFFFFF)
				if sisDir {
					langDir := rsrcRA + sentryOffset
					ln := int(binary.LittleEndian.Uint16(patched[langDir+12:]))
					li := int(binary.LittleEndian.Uint16(patched[langDir+14:]))
					loff := langDir + 16
					for k := 0; k < ln+li; k++ {
						dataOffset := binary.LittleEndian.Uint32(patched[loff+4:])
						isData := (dataOffset & 0x80000000) == 0
						dataEntryOffset := int(dataOffset & 0x7FFFFFFF)
						if isData {
							deOff := rsrcRA + dataEntryOffset
							// Update the size field
							binary.LittleEndian.PutUint32(patched[deOff+4:], uint32(len(iconData)))
						}
						loff += 8
					}
				}
				soff += 8
			}
		}
		off += 8
	}

	_ = rsrcSectionIndex
	_ = rsrcRS
	return patched, nil
}
