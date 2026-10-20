package apkbrand

import (
	"archive/zip"
	"bytes"
	"compress/flate"
	"encoding/binary"
	"errors"
	"hash/crc32"
	"io"
	"strings"
)

// apkEntry is a single member of the APK archive.
type apkEntry struct {
	Name   string
	Method uint16
	Data   []byte
}

// readEntries extracts every member of an APK. The JAR signature files of the source
// archive are skipped because they do not match the rewritten content.
func readEntries(raw []byte) ([]apkEntry, error) {
	reader, err := zip.NewReader(bytes.NewReader(raw), int64(len(raw)))
	if err != nil {
		return nil, err
	}
	entries := make([]apkEntry, 0, len(reader.File))
	for _, file := range reader.File {
		if file.FileInfo().IsDir() {
			continue
		}
		if isJarSignatureEntry(file.Name) {
			continue
		}
		rc, err := file.Open()
		if err != nil {
			return nil, err
		}
		data, err := io.ReadAll(rc)
		rc.Close()
		if err != nil {
			return nil, err
		}
		entries = append(entries, apkEntry{Name: file.Name, Method: file.Method, Data: data})
	}
	return entries, nil
}

// isJarSignatureEntry reports whether the entry belongs to a JAR (v1) signature.
func isJarSignatureEntry(name string) bool {
	upper := strings.ToUpper(name)
	if !strings.HasPrefix(upper, "META-INF/") {
		return false
	}
	base := upper[len("META-INF/"):]
	switch {
	case base == "MANIFEST.MF":
		return true
	case strings.HasSuffix(base, ".SF"), strings.HasSuffix(base, ".RSA"),
		strings.HasSuffix(base, ".DSA"), strings.HasSuffix(base, ".EC"):
		return true
	}
	return false
}

// centralDirectoryRecord describes one entry of the central directory.
type centralDirectoryRecord struct {
	name             string
	method           uint16
	crc              uint32
	compressedSize   uint32
	uncompressedSize uint32
	offset           uint32
	extra            []byte
}

// buildArchive serialises the entries into a ZIP archive. Stored entries are aligned to
// four byte boundaries as required by the Android package installer.
func buildArchive(entries []apkEntry) []byte {
	var buf bytes.Buffer
	records := make([]centralDirectoryRecord, 0, len(entries))

	for _, entry := range entries {
		offset := uint32(buf.Len())
		method := entry.Method
		var payload []byte
		switch method {
		case zip.Store:
			payload = entry.Data
		default:
			payload = deflateBytes(entry.Data)
			if payload == nil {
				method = zip.Store
				payload = entry.Data
			}
		}
		name := []byte(entry.Name)
		extra := alignmentExtra(len(name), method == zip.Store)
		crc := crc32.ChecksumIEEE(entry.Data)

		writeLocalHeader(&buf, method, crc, uint32(len(payload)), uint32(len(entry.Data)), name, extra)
		buf.Write(payload)

		records = append(records, centralDirectoryRecord{
			name:             entry.Name,
			method:           method,
			crc:              crc,
			compressedSize:   uint32(len(payload)),
			uncompressedSize: uint32(len(entry.Data)),
			offset:           offset,
			extra:            extra,
		})
	}

	cdOffset := uint32(buf.Len())
	for _, record := range records {
		writeCentralDirectory(&buf, record)
	}
	cdSize := uint32(buf.Len()) - cdOffset
	writeEndOfCentralDirectory(&buf, uint16(len(records)), cdSize, cdOffset)
	return buf.Bytes()
}

// deflateBytes compresses the payload with the deflate algorithm.
func deflateBytes(data []byte) []byte {
	var out bytes.Buffer
	writer, err := flate.NewWriter(&out, flate.DefaultCompression)
	if err != nil {
		return nil
	}
	if _, err := writer.Write(data); err != nil {
		writer.Close()
		return nil
	}
	if err := writer.Close(); err != nil {
		return nil
	}
	return out.Bytes()
}

// alignmentExtra returns the extra field that aligns the payload of a stored entry.
// The extra field is a sequence of (id, size, payload) blocks; a four byte block with an
// empty payload is the smallest valid one.
func alignmentExtra(nameLen int, stored bool) []byte {
	if !stored {
		return nil
	}
	for pad := 0; pad < 4; pad++ {
		extraLen := 4 + pad
		if (30+nameLen+extraLen)%4 != 0 {
			continue
		}
		extra := make([]byte, extraLen)
		binary.LittleEndian.PutUint16(extra[0:2], 0xd935)
		binary.LittleEndian.PutUint16(extra[2:4], uint16(pad))
		return extra
	}
	return nil
}

// writeLocalHeader writes a ZIP local file header.
func writeLocalHeader(buf *bytes.Buffer, method uint16, crc uint32, compressedSize, uncompressedSize uint32, name, extra []byte) {
	header := make([]byte, 30)
	binary.LittleEndian.PutUint32(header[0:4], 0x04034b50)
	binary.LittleEndian.PutUint16(header[4:6], 20)
	binary.LittleEndian.PutUint16(header[6:8], 0)
	binary.LittleEndian.PutUint16(header[8:10], method)
	binary.LittleEndian.PutUint16(header[10:12], 0)
	binary.LittleEndian.PutUint16(header[12:14], 0)
	binary.LittleEndian.PutUint32(header[14:18], crc)
	binary.LittleEndian.PutUint32(header[18:22], compressedSize)
	binary.LittleEndian.PutUint32(header[22:26], uncompressedSize)
	binary.LittleEndian.PutUint16(header[26:28], uint16(len(name)))
	binary.LittleEndian.PutUint16(header[28:30], uint16(len(extra)))
	buf.Write(header)
	buf.Write(name)
	buf.Write(extra)
}

// writeCentralDirectory writes a ZIP central directory record.
func writeCentralDirectory(buf *bytes.Buffer, record centralDirectoryRecord) {
	name := []byte(record.name)
	header := make([]byte, 46)
	binary.LittleEndian.PutUint32(header[0:4], 0x02014b50)
	binary.LittleEndian.PutUint16(header[4:6], 20)
	binary.LittleEndian.PutUint16(header[6:8], 20)
	binary.LittleEndian.PutUint16(header[8:10], 0)
	binary.LittleEndian.PutUint16(header[10:12], record.method)
	binary.LittleEndian.PutUint16(header[12:14], 0)
	binary.LittleEndian.PutUint16(header[14:16], 0)
	binary.LittleEndian.PutUint32(header[16:20], record.crc)
	binary.LittleEndian.PutUint32(header[20:24], record.compressedSize)
	binary.LittleEndian.PutUint32(header[24:28], record.uncompressedSize)
	binary.LittleEndian.PutUint16(header[28:30], uint16(len(name)))
	binary.LittleEndian.PutUint16(header[30:32], uint16(len(record.extra)))
	binary.LittleEndian.PutUint16(header[32:34], 0)
	binary.LittleEndian.PutUint16(header[34:36], 0)
	binary.LittleEndian.PutUint32(header[36:40], 0)
	binary.LittleEndian.PutUint32(header[42:46], record.offset)
	buf.Write(header)
	buf.Write(name)
	buf.Write(record.extra)
}

// writeEndOfCentralDirectory writes the ZIP end of central directory record.
func writeEndOfCentralDirectory(buf *bytes.Buffer, entries uint16, cdSize, cdOffset uint32) {
	header := make([]byte, 22)
	binary.LittleEndian.PutUint32(header[0:4], 0x06054b50)
	binary.LittleEndian.PutUint16(header[4:6], 0)
	binary.LittleEndian.PutUint16(header[6:8], 0)
	binary.LittleEndian.PutUint16(header[8:10], entries)
	binary.LittleEndian.PutUint16(header[10:12], entries)
	binary.LittleEndian.PutUint32(header[12:16], cdSize)
	binary.LittleEndian.PutUint32(header[16:20], cdOffset)
	binary.LittleEndian.PutUint16(header[20:22], 0)
	buf.Write(header)
}

// findEndOfCentralDirectory returns the offset of the ZIP end of central directory
// record of the given archive.
func findEndOfCentralDirectory(archive []byte) (int, error) {
	const minEOCD = 22
	if len(archive) < minEOCD {
		return 0, errors.New("apkbrand: archive too small")
	}
	limit := len(archive) - minEOCD
	if limit > 0xFFFF {
		limit = 0xFFFF
	}
	for offset := len(archive) - minEOCD; offset >= limit; offset-- {
		if binary.LittleEndian.Uint32(archive[offset:offset+4]) == 0x06054b50 {
			return offset, nil
		}
	}
	return 0, errors.New("apkbrand: end of central directory not found")
}
