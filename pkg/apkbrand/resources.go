package apkbrand

import (
	"encoding/binary"
	"errors"
	"fmt"
	"unicode/utf16"
)

// Resource table constants of the Android binary resource format.
const (
	resTableType        = 0x0002
	resStringPoolType   = 0x0001
	resPackageType      = 0x0200
	resTypeChunkType    = 0x0201
	resUTF8Flag         = 0x100
	resValueTypeString  = 0x03
	resTypeIdString     = 1
	resEntryFlagComplex = 0x0001
)

// stringPool is a parsed ResStringPool chunk.
type stringPool struct {
	start        int
	headerSize   int
	chunkSize    int
	stringCount  int
	styleCount   int
	flags        uint32
	stringsStart int
	stylesStart  int
	offsets      []uint32
}

// utf8Mode reports whether the pool stores its strings in UTF-8.
func (p *stringPool) utf8Mode() bool {
	return p.flags&resUTF8Flag != 0
}

// parseStringPool parses the string pool chunk that starts at the given offset.
func parseStringPool(data []byte, offset int) (*stringPool, error) {
	if offset < 0 || offset+28 > len(data) {
		return nil, errors.New("apkbrand: string pool out of range")
	}
	if binary.LittleEndian.Uint16(data[offset:]) != resStringPoolType {
		return nil, errors.New("apkbrand: not a string pool")
	}
	pool := &stringPool{
		start:        offset,
		headerSize:   int(binary.LittleEndian.Uint16(data[offset+2:])),
		chunkSize:    int(binary.LittleEndian.Uint32(data[offset+4:])),
		stringCount:  int(binary.LittleEndian.Uint32(data[offset+8:])),
		styleCount:   int(binary.LittleEndian.Uint32(data[offset+12:])),
		flags:        binary.LittleEndian.Uint32(data[offset+16:]),
		stringsStart: int(binary.LittleEndian.Uint32(data[offset+20:])),
		stylesStart:  int(binary.LittleEndian.Uint32(data[offset+24:])),
	}
	if pool.chunkSize <= 0 || offset+pool.chunkSize > len(data) {
		return nil, errors.New("apkbrand: string pool out of range")
	}
	pool.offsets = make([]uint32, pool.stringCount)
	base := offset + pool.headerSize
	for i := 0; i < pool.stringCount; i++ {
		if base+i*4+4 > len(data) {
			return nil, errors.New("apkbrand: string offsets out of range")
		}
		pool.offsets[i] = binary.LittleEndian.Uint32(data[base+i*4:])
	}
	return pool, nil
}

// stringAt decodes the string with the given index.
func (p *stringPool) stringAt(data []byte, index int) (string, error) {
	if index < 0 || index >= p.stringCount {
		return "", errors.New("apkbrand: string index out of range")
	}
	offset := p.start + p.stringsStart + int(p.offsets[index])
	value, _, err := decodeString(data, offset, p.utf8Mode())
	return value, err
}

// decodeString decodes a string of a resource string pool and returns the offset of
// the next string.
func decodeString(data []byte, offset int, utf8Mode bool) (string, int, error) {
	if offset < 0 || offset > len(data) {
		return "", 0, errors.New("apkbrand: string out of range")
	}
	if utf8Mode {
		_, next := decodeLength8(data, offset)
		byteLen, next := decodeLength8(data, next)
		if next+byteLen > len(data) {
			return "", 0, errors.New("apkbrand: string out of range")
		}
		return string(data[next : next+byteLen]), next + byteLen, nil
	}
	if offset+2 > len(data) {
		return "", 0, errors.New("apkbrand: string out of range")
	}
	unitLen := int(binary.LittleEndian.Uint16(data[offset:]))
	start := offset + 2
	if start+unitLen*2 > len(data) {
		return "", 0, errors.New("apkbrand: string out of range")
	}
	units := make([]uint16, unitLen)
	for i := 0; i < unitLen; i++ {
		units[i] = binary.LittleEndian.Uint16(data[start+i*2:])
	}
	return string(utf16.Decode(units)), start + unitLen*2, nil
}

// decodeLength8 reads the variable length prefix used by UTF-8 string pools.
func decodeLength8(data []byte, offset int) (int, int) {
	if offset >= len(data) {
		return 0, offset
	}
	value := data[offset]
	offset++
	if value&0x80 != 0 {
		if offset >= len(data) {
			return 0, offset
		}
		length := int(value&0x7f)<<8 | int(data[offset])
		return length, offset + 1
	}
	return int(value), offset
}

// encodeLength8 encodes a length the way UTF-8 string pools expect it.
func encodeLength8(value int) []byte {
	if value < 0x80 {
		return []byte{byte(value)}
	}
	return []byte{byte(0x80 | (value >> 8)), byte(value & 0xff)}
}

// encodeString encodes a value the way the given string pool expects it.
func encodeString(value string, utf8Mode bool) []byte {
	if utf8Mode {
		payload := []byte(value)
		out := make([]byte, 0, len(payload)+8)
		out = append(out, encodeLength8(len(utf16.Encode([]rune(value))))...)
		out = append(out, encodeLength8(len(payload))...)
		out = append(out, payload...)
		return append(out, 0)
	}
	units := utf16.Encode([]rune(value))
	out := make([]byte, 0, len(units)*2+4)
	var length [2]byte
	binary.LittleEndian.PutUint16(length[:], uint16(len(units)))
	out = append(out, length[:]...)
	for _, unit := range units {
		var encoded [2]byte
		binary.LittleEndian.PutUint16(encoded[:], unit)
		out = append(out, encoded[:]...)
	}
	return append(out, 0, 0)
}

// setResourceString rewrites the value of a string resource inside resources.arsc.
func setResourceString(arsc []byte, name string, value string) ([]byte, error) {
	if len(arsc) < 12 {
		return nil, errors.New("apkbrand: resources.arsc too small")
	}
	if binary.LittleEndian.Uint16(arsc) != resTableType {
		return nil, errors.New("apkbrand: not a resource table")
	}
	tableHeaderSize := int(binary.LittleEndian.Uint16(arsc[2:]))
	tableSize := int(binary.LittleEndian.Uint32(arsc[4:]))

	// The global string pool is the first chunk of the table.
	pool, err := parseStringPool(arsc, tableHeaderSize)
	if err != nil {
		return nil, err
	}

	index, err := findStringResourceIndex(arsc, tableHeaderSize, tableSize, pool, name)
	if err != nil {
		return nil, err
	}

	return replaceGlobalString(arsc, pool, index, value)
}

// findStringResourceIndex resolves a resource name to the index of its value inside
// the global string pool.
func findStringResourceIndex(arsc []byte, tableHeaderSize, tableSize int, pool *stringPool, name string) (int, error) {
	offset := tableHeaderSize
	end := tableHeaderSize + tableSize
	for offset < end && offset+8 <= len(arsc) {
		chunkType := binary.LittleEndian.Uint16(arsc[offset:])
		chunkHeaderSize := int(binary.LittleEndian.Uint16(arsc[offset+2:]))
		chunkSize := int(binary.LittleEndian.Uint32(arsc[offset+4:]))
		if chunkSize < 8 || offset+chunkSize > len(arsc) {
			return 0, errors.New("apkbrand: malformed resource table")
		}
		if chunkType == resPackageType {
			index, err := findInPackage(arsc, offset, chunkHeaderSize, chunkSize, pool, name)
			if err == nil {
				return index, nil
			}
			if !errors.Is(err, errResourceNotFound) {
				return 0, err
			}
		}
		offset += chunkSize
	}
	return 0, fmt.Errorf("apkbrand: resource %q not found", name)
}

// errResourceNotFound signals that a resource name does not exist in a package.
var errResourceNotFound = errors.New("apkbrand: resource not found")

// findInPackage looks for the resource inside a resource package chunk.
func findInPackage(arsc []byte, packageOffset, packageHeaderSize, packageSize int, pool *stringPool, name string) (int, error) {
	if packageOffset+packageHeaderSize > len(arsc) {
		return 0, errResourceNotFound
	}
	keyStringsOffset := int(binary.LittleEndian.Uint32(arsc[packageOffset+276:]))
	typeStringsOffset := int(binary.LittleEndian.Uint32(arsc[packageOffset+268:]))
	if keyStringsOffset <= 0 || packageOffset+keyStringsOffset >= len(arsc) {
		return 0, errResourceNotFound
	}
	keyPool, err := parseStringPool(arsc, packageOffset+keyStringsOffset)
	if err != nil {
		return 0, errResourceNotFound
	}
	var typePool *stringPool
	if typeStringsOffset > 0 && packageOffset+typeStringsOffset < len(arsc) {
		typePool, _ = parseStringPool(arsc, packageOffset+typeStringsOffset)
	}

	offset := packageOffset + packageHeaderSize
	end := packageOffset + packageSize
	for offset < end && offset+8 <= len(arsc) {
		chunkType := binary.LittleEndian.Uint16(arsc[offset:])
		chunkHeaderSize := int(binary.LittleEndian.Uint16(arsc[offset+2:]))
		chunkSize := int(binary.LittleEndian.Uint32(arsc[offset+4:]))
		if chunkSize < 8 || offset+chunkSize > len(arsc) {
			return 0, errResourceNotFound
		}
		if chunkType == resTypeChunkType && chunkHeaderSize >= 20 {
			// The type id of a chunk is one based, the name is stored in the type name
			// pool of the package at the position of the type id minus one.
			typeID := int(arsc[offset+8])
			isStringType := typePool == nil
			if typePool != nil && typeID-1 >= 0 && typeID-1 < typePool.stringCount {
				typeName, err := typePool.stringAt(arsc, typeID-1)
				isStringType = err == nil && typeName == "string"
			}
			if isStringType {
				index, err := findInTypeChunk(arsc, offset, chunkHeaderSize, chunkSize, keyPool, pool, name)
				if err == nil {
					return index, nil
				}
				if !errors.Is(err, errResourceNotFound) {
					return 0, err
				}
			}
		}
		offset += chunkSize
	}
	return 0, errResourceNotFound
}

// findInTypeChunk scans the entries of a type chunk for the requested resource.
func findInTypeChunk(arsc []byte, chunkOffset, chunkHeaderSize, chunkSize int, keyPool, pool *stringPool, name string) (int, error) {
	entryCount := int(binary.LittleEndian.Uint32(arsc[chunkOffset+12:]))
	entriesStart := int(binary.LittleEndian.Uint32(arsc[chunkOffset+16:]))
	entryOffset := chunkOffset + entriesStart
	limit := chunkOffset + chunkSize
	if limit > len(arsc) {
		limit = len(arsc)
	}
	for i := 0; i < entryCount; i++ {
		if entryOffset+8 > limit {
			return 0, errResourceNotFound
		}
		entrySize := int(binary.LittleEndian.Uint16(arsc[entryOffset:]))
		flags := binary.LittleEndian.Uint16(arsc[entryOffset+2:])
		key := int(binary.LittleEndian.Uint32(arsc[entryOffset+4:]))
		if entrySize < 8 {
			return 0, errResourceNotFound
		}
		valueOffset := entryOffset + entrySize
		if flags&resEntryFlagComplex != 0 {
			// Complex entries hold a map instead of a value.
			entryOffset = valueOffset
			continue
		}
		if valueOffset+8 > len(arsc) {
			return 0, errResourceNotFound
		}
		valueSize := int(binary.LittleEndian.Uint16(arsc[valueOffset:]))
		if valueSize < 8 {
			return 0, errResourceNotFound
		}
		next := valueOffset + valueSize

		keyName, err := keyPool.stringAt(arsc, key)
		if err == nil && keyName == name {
			dataType := arsc[valueOffset+3]
			data := binary.LittleEndian.Uint32(arsc[valueOffset+4:])
			if dataType != resValueTypeString {
				return 0, errResourceNotFound
			}
			return int(data), nil
		}
		entryOffset = next
	}
	return 0, errResourceNotFound
}

// replaceGlobalString rewrites a single entry of the global string pool. When the new
// value does not fit into the existing slot the pool is rebuilt, which changes the size
// of the resource table.
func replaceGlobalString(arsc []byte, pool *stringPool, index int, value string) ([]byte, error) {
	if index < 0 || index >= pool.stringCount {
		return nil, errors.New("apkbrand: string index out of range")
	}
	encoded := encodeString(value, pool.utf8Mode())

	dataStart := pool.start + pool.stringsStart
	dataEnd := pool.start + pool.chunkSize
	if pool.stylesStart > 0 {
		dataEnd = pool.start + pool.stylesStart
	}

	// Raw span of every string inside the pool.
	spans := make([][2]int, pool.stringCount)
	for i := 0; i < pool.stringCount; i++ {
		start := dataStart + int(pool.offsets[i])
		end := dataEnd
		if i+1 < pool.stringCount {
			end = dataStart + int(pool.offsets[i+1])
		}
		spans[i] = [2]int{start, end}
	}

	target := spans[index]
	oldLen := target[1] - target[0]
	if len(encoded) <= oldLen {
		out := make([]byte, len(arsc))
		copy(out, arsc)
		copy(out[target[0]:], encoded)
		return out, nil
	}

	// Rebuild the string data area with the longer value.
	delta := len(encoded) - oldLen
	newData := make([]byte, 0, dataEnd-dataStart+delta)
	newOffsets := make([]uint32, pool.stringCount)
	position := 0
	for i := 0; i < pool.stringCount; i++ {
		newOffsets[i] = uint32(position)
		if i == index {
			newData = append(newData, encoded...)
			position += len(encoded)
			continue
		}
		chunk := arsc[spans[i][0]:spans[i][1]]
		newData = append(newData, chunk...)
		position += len(chunk)
	}

	// The resource table is aligned to four byte boundaries, therefore the string
	// data area is padded to keep the size of the table aligned as well.
	if padding := (4 - len(newData)%4) % 4; padding > 0 {
		newData = append(newData, make([]byte, padding)...)
		delta += padding
	}

	newPoolSize := pool.chunkSize + delta
	newPool := make([]byte, 0, newPoolSize)
	header := make([]byte, pool.headerSize)
	copy(header, arsc[pool.start:pool.start+pool.headerSize])
	binary.LittleEndian.PutUint32(header[4:], uint32(newPoolSize))
	if pool.stylesStart > 0 {
		binary.LittleEndian.PutUint32(header[24:], uint32(pool.stylesStart+delta))
	}
	newPool = append(newPool, header...)

	offsetBase := len(newPool)
	for _, offset := range newOffsets {
		encodedOffset := make([]byte, 4)
		binary.LittleEndian.PutUint32(encodedOffset, offset)
		newPool = append(newPool, encodedOffset...)
	}
	for i := 0; i < pool.styleCount; i++ {
		styleOffset := arsc[pool.start+pool.headerSize+pool.stringCount*4+i*4:]
		newPool = append(newPool, styleOffset...)
	}
	if pool.stylesStart > 0 {
		// The string data must start at the same offset as before.
		if offsetBase+pool.stringCount*4+pool.styleCount*4 != pool.stringsStart {
			return nil, errors.New("apkbrand: unexpected string pool layout")
		}
	}
	newPool = append(newPool, newData...)
	if pool.stylesStart > 0 {
		newPool = append(newPool, arsc[pool.start+pool.stylesStart:pool.start+pool.chunkSize]...)
	}

	out := make([]byte, 0, len(arsc)+delta)
	out = append(out, arsc[:pool.start]...)
	out = append(out, newPool...)
	out = append(out, arsc[pool.start+pool.chunkSize:]...)
	binary.LittleEndian.PutUint32(out[4:], uint32(len(out)))
	return out, nil
}
