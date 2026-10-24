package embedcfg

import (
	"encoding/binary"
	"fmt"
)

type peIconResource struct {
	id          uint32
	dataOffset  int
	dataSize    int
	entryOffset int
}

// PatchIcon replaces the icon resource in a Windows PE file.
//
// The icon of the installation is embedded into the program of the client, the
// program carries the name and the icon of the installation. The client does
// not ask the user for them.
//
// The icon data is a PNG picture. The picture is written into the resource of
// the program in place of the picture that was there, the program does not grow
// with it. The group icon is rewritten so Explorer always uses the new picture
// instead of one of the old sizes.
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
	sectionTableOffset := peOffset + 24 + optHeaderSize

	// Find the .rsrc section.
	var rsrcRA, rsrcVA int
	found := false
	for i := 0; i < numSections; i++ {
		off := sectionTableOffset + i*40
		if off+40 > len(program) {
			break
		}
		name := string(program[off : off+8])
		for len(name) > 0 && name[len(name)-1] == 0 {
			name = name[:len(name)-1]
		}
		if name == ".rsrc" {
			rsrcVA = int(binary.LittleEndian.Uint32(program[off+12:]))
			rsrcRA = int(binary.LittleEndian.Uint32(program[off+20:]))
			found = true
			break
		}
	}
	if !found {
		return nil, fmt.Errorf("the resource section is not found")
	}
	if rsrcRA < 0 || rsrcRA+16 > len(program) {
		return nil, fmt.Errorf("the resource section is outside the program")
	}

	// Parse the root resource directory.
	numNamed := int(binary.LittleEndian.Uint16(program[rsrcRA+12:]))
	numID := int(binary.LittleEndian.Uint16(program[rsrcRA+14:]))
	off := rsrcRA + 16

	var icons []peIconResource
	var groupIconOffset, groupIconSize int

	for i := 0; i < numNamed+numID; i++ {
		if off+8 > len(program) {
			return nil, fmt.Errorf("the resource directory is truncated")
		}
		typeID := binary.LittleEndian.Uint32(program[off:])
		nameOffset := binary.LittleEndian.Uint32(program[off+4:])
		isDir := (nameOffset & 0x80000000) != 0
		entryOffset := int(nameOffset & 0x7FFFFFFF)
		if typeID == 3 && isDir { // RT_ICON
			subDir := rsrcRA + entryOffset
			if subDir+16 > len(program) {
				return nil, fmt.Errorf("the icon directory is truncated")
			}
			sn := int(binary.LittleEndian.Uint16(program[subDir+12:]))
			si := int(binary.LittleEndian.Uint16(program[subDir+14:]))
			soff := subDir + 16
			for j := 0; j < sn+si; j++ {
				if soff+8 > len(program) {
					return nil, fmt.Errorf("the icon entry is truncated")
				}
				iconID := binary.LittleEndian.Uint32(program[soff:])
				soffset := binary.LittleEndian.Uint32(program[soff+4:])
				sisDir := (soffset & 0x80000000) != 0
				sentryOffset := int(soffset & 0x7FFFFFFF)
				if sisDir {
					langDir := rsrcRA + sentryOffset
					if langDir+16 > len(program) {
						return nil, fmt.Errorf("the icon language directory is truncated")
					}
					ln := int(binary.LittleEndian.Uint16(program[langDir+12:]))
					li := int(binary.LittleEndian.Uint16(program[langDir+14:]))
					loff := langDir + 16
					for k := 0; k < ln+li; k++ {
						if loff+8 > len(program) {
							return nil, fmt.Errorf("the icon language entry is truncated")
						}
						dataOffset := binary.LittleEndian.Uint32(program[loff+4:])
						if (dataOffset & 0x80000000) == 0 {
							deOff := rsrcRA + int(dataOffset&0x7FFFFFFF)
							if deOff+8 > len(program) {
								return nil, fmt.Errorf("the icon data entry is truncated")
							}
							dataRVA := binary.LittleEndian.Uint32(program[deOff:])
							dataSize := int(binary.LittleEndian.Uint32(program[deOff+4:]))
							rawOff := int(dataRVA) - rsrcVA + rsrcRA
							if rawOff >= 0 && rawOff+dataSize <= len(program) {
								icons = append(icons, peIconResource{
									id:          iconID,
									dataOffset:  rawOff,
									dataSize:    dataSize,
									entryOffset: deOff,
								})
							}
						}
						loff += 8
					}
				}
				soff += 8
			}
		}
		if typeID == 14 && isDir { // RT_GROUP_ICON
			subDir := rsrcRA + entryOffset
			if subDir+16 > len(program) {
				return nil, fmt.Errorf("the group icon directory is truncated")
			}
			sn := int(binary.LittleEndian.Uint16(program[subDir+12:]))
			si := int(binary.LittleEndian.Uint16(program[subDir+14:]))
			soff := subDir + 16
			for j := 0; j < sn+si; j++ {
				if soff+8 > len(program) {
					return nil, fmt.Errorf("the group icon entry is truncated")
				}
				soffset := binary.LittleEndian.Uint32(program[soff+4:])
				sisDir := (soffset & 0x80000000) != 0
				sentryOffset := int(soffset & 0x7FFFFFFF)
				if sisDir {
					langDir := rsrcRA + sentryOffset
					if langDir+16 > len(program) {
						return nil, fmt.Errorf("the group icon language directory is truncated")
					}
					ln := int(binary.LittleEndian.Uint16(program[langDir+12:]))
					li := int(binary.LittleEndian.Uint16(program[langDir+14:]))
					loff := langDir + 16
					for k := 0; k < ln+li; k++ {
						if loff+8 > len(program) {
							return nil, fmt.Errorf("the group icon language entry is truncated")
						}
						dataOffset := binary.LittleEndian.Uint32(program[loff+4:])
						if (dataOffset & 0x80000000) == 0 {
							deOff := rsrcRA + int(dataOffset&0x7FFFFFFF)
							if deOff+8 > len(program) {
								return nil, fmt.Errorf("the group icon data entry is truncated")
							}
							dataRVA := binary.LittleEndian.Uint32(program[deOff:])
							dataSize := int(binary.LittleEndian.Uint32(program[deOff+4:]))
							rawOff := int(dataRVA) - rsrcVA + rsrcRA
							if rawOff >= 0 && rawOff+dataSize <= len(program) {
								groupIconOffset = rawOff
								groupIconSize = dataSize
							}
						}
						loff += 8
					}
				}
				soff += 8
			}
		}
		off += 8
	}

	if len(icons) == 0 {
		return nil, fmt.Errorf("the icon resource is not found")
	}
	if groupIconOffset == 0 || groupIconSize < 20 {
		return nil, fmt.Errorf("the group icon resource is not found")
	}

	// Use the largest existing icon slot so the picture fits without growing
	// the program.
	chosen := icons[0]
	for _, item := range icons[1:] {
		if item.dataSize > chosen.dataSize {
			chosen = item
		}
	}
	if len(iconData) > chosen.dataSize {
		return nil, fmt.Errorf("the icon is too large: %d > %d", len(iconData), chosen.dataSize)
	}

	// Make a copy of the program.
	patched := make([]byte, len(program))
	copy(patched, program)

	// Replace the icon data and pad the rest of the slot with zeros.
	for i := 0; i < chosen.dataSize; i++ {
		if i < len(iconData) {
			patched[chosen.dataOffset+i] = iconData[i]
		} else {
			patched[chosen.dataOffset+i] = 0
		}
	}

	// Update the group icon so it has one entry, the one of the new picture.
	binary.LittleEndian.PutUint16(patched[groupIconOffset+4:], 1)
	for index := 6; index < 20; index++ {
		patched[groupIconOffset+index] = 0
	}
	if len(iconData) >= 24 && string(iconData[1:4]) == "PNG" {
		width := binary.BigEndian.Uint32(iconData[16:20])
		height := binary.BigEndian.Uint32(iconData[20:24])
		if width < 256 {
			patched[groupIconOffset+6] = byte(width)
		}
		if height < 256 {
			patched[groupIconOffset+7] = byte(height)
		}
	}
	binary.LittleEndian.PutUint16(patched[groupIconOffset+10:], 1)
	binary.LittleEndian.PutUint16(patched[groupIconOffset+12:], 32)
	binary.LittleEndian.PutUint32(patched[groupIconOffset+14:], uint32(len(iconData)))
	binary.LittleEndian.PutUint16(patched[groupIconOffset+18:], uint16(chosen.id))

	// Update only the data entry of the icon that was replaced. The other
	// pictures keep their sizes.
	binary.LittleEndian.PutUint32(patched[chosen.entryOffset+4:], uint32(len(iconData)))

	return patched, nil
}
