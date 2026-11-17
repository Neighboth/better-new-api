/*
concat_mp4.go

Concatenates MP4 files produced by the same encoder into a single file,
without re-encoding. The parts of a chained Aepi generation are produced by
the same model on the same engine, so codec configuration and timescales are
identical across parts; this file verifies that and falls back fast when they
are not (callers fall back to the browser merge).

Output layout:

	ftyp (verbatim from the first part)
	moov (rebuilt: mvhd, per track tkhd/mdhd/hdlr, minf(vmhd|smhd, dinf,
	      stbl(stsd copied, stsz/stts/ctts?/stsc/stco/stss? rebuilt)))
	mdat (track payloads in track order, parts in order within a track)

Edit lists are intentionally dropped: the merged output is a plain sequential
playback timeline.
*/
package service

import (
	"bytes"
	"encoding/binary"
	"errors"
	"fmt"
	"os"
)

const (
	boxHeaderSize = 8

	mvhdType = "mvhd"
	mdhdType = "mdhd"
	tkhdType = "tkhd"
	hdlrType = "hdlr"
	stblType = "stbl"
	stsdType = "stsd"
	stszType = "stsz"
	sttsType = "stts"
	cttsType = "ctts"
	stscType = "stsc"
	stcoType = "stco"
	stssType = "stss"
	vmhdType = "vmhd"
	smhdType = "smhd"
	dinfType = "dinf"
)

var containerBoxes = map[string]bool{
	"moov": true, "trak": true, "mdia": true, "minf": true,
	"stbl": true, "dinf": true,
}

type rawBox struct {
	typ    string
	offset uint64
	size   uint64
	header uint64
}

func readUint32(b []byte) uint32       { return binary.BigEndian.Uint32(b) }
func readUint64(b []byte) uint64       { return binary.BigEndian.Uint64(b) }
func writeUint16(dst []byte, v uint16) { binary.BigEndian.PutUint16(dst, v) }
func writeUint32(dst []byte, v uint32) { binary.BigEndian.PutUint32(dst, v) }
func writeUint64(dst []byte, v uint64) { binary.BigEndian.PutUint64(dst, v) }

func walkBoxes(data []byte, base uint64, out *[]rawBox) {
	offset := uint64(0)
	for offset+boxHeaderSize <= uint64(len(data)) {
		size := uint64(readUint32(data[offset:]))
		typ := string(data[offset+4 : offset+8])
		header := uint64(boxHeaderSize)
		if size == 1 {
			size = readUint64(data[offset+8:])
			header = 16
		}
		if size == 0 {
			size = uint64(len(data)) - offset
		}
		if size < header || offset+size > uint64(len(data)) {
			return
		}
		*out = append(*out, rawBox{typ, base + offset, size, header})
		if containerBoxes[typ] {
			walkBoxes(data[offset+header:offset+size], base+offset+header, out)
		}
		offset += size
	}
}

func locateBox(boxes []rawBox, typ string) (rawBox, bool) {
	for _, b := range boxes {
		if b.typ == typ {
			return b, true
		}
	}
	return rawBox{}, false
}

func locateAllBoxes(boxes []rawBox, typ string) []rawBox {
	found := []rawBox{}
	for _, b := range boxes {
		if b.typ == typ {
			found = append(found, b)
		}
	}
	return found
}

func boxBytes(data []byte, box rawBox) []byte {
	return data[box.offset : box.offset+box.size]
}

// ---------- Source parts ----------

type sourceTrack struct {
	id            uint32
	timescale     uint32
	duration      uint64 // in track timescale
	isVideo       bool
	mvhdDuration  uint64 // movie timescale duration contribution (per part, same across tracks)
	mvhdTimescale uint32
	mvhdCreation  uint32
	mvhdModTime   uint32
	mvhdMatrix    []byte

	sampleSizes []uint32
	sampleDelta []uint32
	sampleComp  []int64  // nil when the part has no ctts
	syncSamples []uint32 // 1-based; nil when the part has no stss

	chunks          []uint64 // chunk data offsets
	samplesPerChunk []uint32 // expanded per chunk
	stsdBytes       []byte   // verbatim stsd
	hdlrBytes       []byte   // verbatim hdlr
	vmhdBytes       []byte   // verbatim (video only)
	smhdBytes       []byte   // verbatim (audio only)
	dinfBytes       []byte   // verbatim dinf
	tkhdBytes       []byte   // verbatim tkhd (template; duration patched per output)
	videoWidth      uint32   // fixed point 16.16
	videoHeight     uint32
}

// ConcatMP4Files concatenates the given MP4 files into outPath.
func ConcatMP4Files(paths []string, outPath string) error {
	if len(paths) < 2 {
		return errors.New("ConcatMP4Files: at least two parts are required")
	}
	parts := make([]*sourcePart, 0, len(paths))
	for _, path := range paths {
		p, err := openSourcePart(path)
		if err != nil {
			return fmt.Errorf("ConcatMP4Files: %s: %w", path, err)
		}
		parts = append(parts, p)
	}
	for i := 1; i < len(parts); i++ {
		if err := checkConcatCompatibility(parts[0], parts[i]); err != nil {
			return fmt.Errorf("ConcatMP4Files: %s: %w", paths[i], err)
		}
	}
	return writeConcat(parts, outPath)
}

// sourcePart is one input MP4 file.
type sourcePart struct {
	data   []byte
	top    []rawBox
	tracks []*sourceTrack
}

func openSourcePart(path string) (*sourcePart, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	p := &sourcePart{data: data, top: []rawBox{}}
	walkBoxes(data, 0, &p.top)
	mvhdBox, ok := locateBox(p.top, mvhdType)
	if !ok {
		return nil, errors.New("mvhd not found")
	}
	mbuf := data[mvhdBox.offset : mvhdBox.offset+mvhdBox.size]
	if len(mbuf) != 108 || mbuf[8] != 0 {
		return nil, errors.New("only v0 mvhd is supported")
	}
	traks := locateAllBoxes(p.top, "trak")
	if len(traks) == 0 {
		return nil, errors.New("no trak boxes")
	}
	for ti, trak := range traks {
		track, err := readSourceTrack(data, trak, ti, mbuf)
		if err != nil {
			return nil, fmt.Errorf("trak %d: %w", ti, err)
		}
		p.tracks = append(p.tracks, track)
	}
	return p, nil
}

func readSourceTrack(data []byte, trak rawBox, ti int, mvhd []byte) (*sourceTrack, error) {
	sub := []rawBox{}
	walkBoxes(data[trak.offset+trak.header:trak.offset+trak.size], trak.offset+trak.header, &sub)
	mdhdBox, ok := locateBox(sub, mdhdType)
	if !ok {
		return nil, errors.New("mdhd not found")
	}
	stblBoxes := locateAllBoxes(sub, stblType)
	if len(stblBoxes) == 0 {
		return nil, errors.New("stbl not found")
	}
	sub2 := []rawBox{}
	for _, s := range stblBoxes {
		walkBoxes(data[s.offset+s.header:s.offset+s.size], s.offset+s.header, &sub2)
	}

	t := &sourceTrack{}

	mbuf := data[mdhdBox.offset : mdhdBox.offset+mdhdBox.size]
	if len(mbuf) != 32 || mbuf[8] != 0 {
		return nil, errors.New("unsupported mdhd format")
	}
	t.timescale = readUint32(mbuf[20:])
	t.duration = uint64(readUint32(mbuf[24:]))

	// movie-level fields are attached to every track for convenience
	t.mvhdTimescale = readUint32(mvhd[20:])
	t.mvhdDuration = uint64(readUint32(mvhd[24:]))
	t.mvhdCreation = readUint32(mvhd[12:])
	t.mvhdModTime = readUint32(mvhd[16:])
	t.mvhdMatrix = append([]byte{}, mvhd[44:80]...)

	if stszBox, ok := locateBox(sub2, stszType); ok {
		sbuf := data[stszBox.offset : stszBox.offset+stszBox.size]
		if len(sbuf) < 20 {
			return nil, errors.New("stsz too small")
		}
		sampleCount := readUint32(sbuf[16:])
		t.sampleSizes = make([]uint32, sampleCount)
		for i := uint32(0); i < sampleCount && 20+i*4 < uint32(len(sbuf)); i++ {
			t.sampleSizes[i] = readUint32(sbuf[20+i*4:])
		}
	} else {
		return nil, errors.New("stsz not found")
	}

	if sttsBox, ok := locateBox(sub2, sttsType); ok {
		tbuf := data[sttsBox.offset : sttsBox.offset+sttsBox.size]
		count := readUint32(tbuf[12:])
		for i := uint32(0); i < count; i++ {
			off := uint32(16 + i*8)
			if off+8 > uint32(len(tbuf)) {
				return nil, errors.New("stts truncated")
			}
			n := readUint32(tbuf[off:])       // sample_count
			delta := readUint32(tbuf[off+4:]) // sample_delta
			for j := uint32(0); j < n; j++ {
				t.sampleDelta = append(t.sampleDelta, delta)
			}
		}
	} else {
		return nil, errors.New("stts not found")
	}
	if uint32(len(t.sampleDelta)) != uint32(len(t.sampleSizes)) {
		return nil, fmt.Errorf("stts/stsz sample count mismatch (%d vs %d)", len(t.sampleDelta), len(t.sampleSizes))
	}

	if cttsBox, ok := locateBox(sub2, cttsType); ok {
		cbuf := data[cttsBox.offset : cttsBox.offset+cttsBox.size]
		count := readUint32(cbuf[12:])
		for i := uint32(0); i < count; i++ {
			off := uint32(16 + i*8)
			if off+8 > uint32(len(cbuf)) {
				return nil, errors.New("ctts truncated")
			}
			n := readUint32(cbuf[off:])
			offset := int64(int32(readUint32(cbuf[off+4:])))
			for j := uint32(0); j < n; j++ {
				t.sampleComp = append(t.sampleComp, offset)
			}
		}
		if len(t.sampleComp) != len(t.sampleSizes) {
			return nil, errors.New("ctts sample count mismatch")
		}
	}

	if stssBox, ok := locateBox(sub2, stssType); ok {
		sbuf := data[stssBox.offset : stssBox.offset+stssBox.size]
		count := readUint32(sbuf[12:])
		for i := uint32(0); i < count; i++ {
			off := uint32(16 + i*4)
			if off+4 > uint32(len(sbuf)) {
				return nil, errors.New("stss truncated")
			}
			t.syncSamples = append(t.syncSamples, readUint32(sbuf[off:]))
		}
	}

	// stsc + stco to be able to copy sample payloads
	if stscBox, ok := locateBox(sub2, stscType); ok {
		sbuf := data[stscBox.offset : stscBox.offset+stscBox.size]
		count := readUint32(sbuf[12:])
		type entry struct {
			firstChunk uint32
			samples    uint32
		}
		entries := make([]entry, 0, count)
		for i := uint32(0); i < count; i++ {
			off := uint32(16 + i*12)
			if off+12 > uint32(len(sbuf)) {
				return nil, errors.New("stsc truncated")
			}
			entries = append(entries, entry{readUint32(sbuf[off:]), readUint32(sbuf[off+4:])})
		}

		if stcoBox, ok := locateBox(sub2, stcoType); ok {
			cbuf := data[stcoBox.offset : stcoBox.offset+stcoBox.size]
			chunkCount := readUint32(cbuf[12:])
			offsets := make([]uint64, 0, chunkCount)
			for i := uint32(0); i < chunkCount; i++ {
				off := uint32(16 + i*4)
				if off+4 > uint32(len(cbuf)) {
					return nil, errors.New("stco truncated")
				}
				offsets = append(offsets, uint64(readUint32(cbuf[off:])))
			}
			pos := uint32(0)
			part := uint32(0) // index into entries
			for ci := uint32(0); ci < chunkCount; ci++ {
				for part+1 < uint32(len(entries)) && entries[part+1].firstChunk == ci+1 {
					part++
				}
				t.samplesPerChunk = append(t.samplesPerChunk, entries[part].samples)
				pos += entries[part].samples
			}
			if pos != uint32(len(t.sampleSizes)) {
				return nil, fmt.Errorf("stsc expands to %d samples, stsz has %d", pos, len(t.sampleSizes))
			}
			t.chunks = offsets
		} else {
			return nil, errors.New("stco not found")
		}
	} else {
		return nil, errors.New("stsc not found")
	}

	// raw sub-boxes to copy verbatim
	copify := func(typ string, dst *[]byte) error {
		box, ok := locateBox(sub, typ)
		if ok {
			*dst = append([]byte{}, data[box.offset:box.offset+box.size]...)
			return nil
		}
		box, ok = locateBox(sub2, typ)
		if ok {
			*dst = append([]byte{}, data[box.offset:box.offset+box.size]...)
			return nil
		}
		return errors.New(typ + " not found")
	}
	if err := copify(stsdType, &t.stsdBytes); err != nil {
		return nil, err
	}
	if err := copify(hdlrType, &t.hdlrBytes); err != nil {
		return nil, err
	}
	if t.isVideoTrack(t.stsdBytes) {
		if err := copify(vmhdType, &t.vmhdBytes); err != nil {
			return nil, err
		}
	} else {
		if err := copify(smhdType, &t.smhdBytes); err != nil {
			return nil, err
		}
	}
	if err := copify(dinfType, &t.dinfBytes); err != nil {
		return nil, err
	}
	if err := copify(tkhdType, &t.tkhdBytes); err != nil {
		return nil, err
	}

	// width/height from tkhd (v0 offsets 84/88)
	if t.isVideoTrack(t.stsdBytes) {
		buf := t.tkhdBytes
		if len(buf) < 92 {
			return nil, errors.New("tkhd box too small for dimensions")
		}
		t.videoWidth = readUint32(buf[84:])
		t.videoHeight = readUint32(buf[88:])
	}

	tkbuf2 := t.tkhdBytes
	if len(tkbuf2) < 24 {
		return nil, errors.New("tkhd box too small")
	}
	t.id = readUint32(tkbuf2[20:])
	t.isVideo = t.isVideoTrack(t.stsdBytes)
	return t, nil
}

func (*sourceTrack) isVideoTrack(stsd []byte) bool {
	if len(stsd) < 24 {
		return false
	}
	switch string(stsd[20:24]) {
	case "avc1", "hvc1", "hev1", "encv", "mp4v":
		return true
	}
	return false
}

func checkConcatCompatibility(a, b *sourcePart) error {
	if len(a.tracks) != len(b.tracks) {
		return fmt.Errorf("track count mismatch (%d vs %d)", len(a.tracks), len(b.tracks))
	}
	for i := range a.tracks {
		ta, tb := a.tracks[i], b.tracks[i]
		if ta.isVideo != tb.isVideo {
			return fmt.Errorf("track %d video/audio mismatch", i)
		}
		if ta.timescale != tb.timescale {
			return fmt.Errorf("track %d timescale mismatch (%d vs %d)", i, ta.timescale, tb.timescale)
		}
		fa, err := codecFingerprint(ta.stsdBytes)
		if err != nil {
			return fmt.Errorf("track %d: %w", i, err)
		}
		fb, err := codecFingerprint(tb.stsdBytes)
		if err != nil {
			return fmt.Errorf("track %d: %w", i, err)
		}
		if !bytes.Equal(fa, fb) {
			return fmt.Errorf("track %d codec configuration mismatch", i)
		}
		if ta.id != tb.id {
			return fmt.Errorf("track %d id mismatch (%d vs %d)", i, ta.id, tb.id)
		}
		if ta.isVideo {
			if ta.videoWidth != tb.videoWidth || ta.videoHeight != tb.videoHeight {
				return fmt.Errorf("track %d dimensions mismatch", i)
			}
		}
		if (ta.sampleComp == nil) != (tb.sampleComp == nil) {
			return fmt.Errorf("track %d ctts mismatch", i)
		}
		if (ta.syncSamples == nil) != (tb.syncSamples == nil) {
			return fmt.Errorf("track %d stss mismatch", i)
		}
	}
	if a.tracks[0].mvhdTimescale != b.tracks[0].mvhdTimescale {
		return fmt.Errorf("movie timescale mismatch (%d vs %d)", a.tracks[0].mvhdTimescale, b.tracks[0].mvhdTimescale)
	}
	return nil
}

// codecFingerprint extracts the decoder-essential bytes of an stsd box.
// Average bitrate is set by the encoder from content and drifts between runs,
// so it is excluded — same encoder settings always yield identical fingerprints.
func codecFingerprint(stsd []byte) ([]byte, error) {
	if len(stsd) < 24 {
		return nil, errors.New("stsd box too small")
	}
	entryType := string(stsd[20:24])
	fp := append([]byte{}, entryType...)
	switch entryType {
	case "mp4a":
		if len(stsd) < 52 {
			return nil, errors.New("mp4a entry too small")
		}
		fp = append(fp, stsd[40:42]...) // channel_count
		fp = append(fp, stsd[42:44]...) // sample_size
		fp = append(fp, stsd[48:52]...) // sample_rate
		asc, err := decoderSpecificInfo(stsd)
		if err != nil {
			return nil, err
		}
		fp = append(fp, asc...)
		return fp, nil
	case "avc1", "hvc1", "hev1":
		cfg := findEmbeddedBox(stsd, map[string]bool{"avcC": true, "hvcC": true, "hevC": true})
		if cfg == nil {
			return nil, errors.New("codec configuration box not found")
		}
		fp = append(fp, cfg...)
		return fp, nil
	default:
		return append(fp, stsd[24:]...), nil
	}
}

// readEsdsTLV reads one ISO-14496-1 TLV field: tag (1 byte) + size (1-4 bytes,
// 7 bits each, MSB = continuation) + payload.
func readEsdsTLV(s []byte, off, end int) (tag byte, payload []byte, next int, ok bool) {
	if off < 0 || off >= end {
		return 0, nil, off, false
	}
	tag = s[off]
	size := 0
	i := off + 1
	for {
		if i >= end {
			return 0, nil, off, false
		}
		b := s[i]
		i++
		size = (size << 7) | int(b&0x7f)
		if b&0x80 == 0 {
			break
		}
	}
	if i+size > end {
		return 0, nil, off, false
	}
	return tag, s[i : i+size], i + size, true
}

// decoderSpecificInfo returns the AudioSpecificConfig payload of an mp4a
// stsd box by walking its esds descriptor chain.
func decoderSpecificInfo(stsd []byte) ([]byte, error) {
	idx := bytes.Index(stsd, []byte("esds"))
	if idx < 4 {
		return nil, errors.New("esds box not found")
	}
	boxStart := idx - 4
	boxSize := uint64(readUint32(stsd[boxStart:]))
	boxEnd := int(uint64(boxStart) + boxSize)
	if boxEnd > len(stsd) {
		return nil, errors.New("esds box out of range")
	}
	tag, es, _, ok := readEsdsTLV(stsd, idx+4+4, boxEnd) // skip fourcc + version/flags
	if !ok || tag != 0x03 {
		return nil, errors.New("ES_Descriptor not found")
	}
	if len(es) < 3 {
		return nil, errors.New("ES_Descriptor too small")
	}
	q := 3 // ES_ID (2) + flags (1)
	flags := es[2]
	if flags&0x80 != 0 {
		q += 2 // streamDependenceFlag -> depends_on_es_id
	}
	if flags&0x40 != 0 { // URL_Flag
		if q >= len(es) {
			return nil, errors.New("ES_Descriptor truncated (url)")
		}
		q += 1 + int(es[q])
	}
	tag, dcfg, _, ok := readEsdsTLV(es, q, len(es))
	if !ok || tag != 0x04 {
		return nil, errors.New("DecoderConfigDescriptor not found")
	}
	if len(dcfg) < 13 {
		return nil, errors.New("DecoderConfigDescriptor too small")
	}
	// objectTypeIndication(1) + streamType(1) + bufferSizeDB(3) + maxBitRate(4) + avgBitRate(4)
	tag, asc, _, ok := readEsdsTLV(dcfg, 13, len(dcfg))
	if !ok || tag != 0x05 {
		return nil, errors.New("DecoderSpecificInfo not found")
	}
	return asc, nil
}

// findEmbeddedBox locates a codec config box by fourcc inside a stsd box and
// returns its raw bytes, header included.
func findEmbeddedBox(stsd []byte, fourccs map[string]bool) []byte {
	for name := range fourccs {
		idx := bytes.Index(stsd, []byte(name))
		if idx < 4 {
			continue
		}
		size := uint64(readUint32(stsd[idx-4:]))
		if size < 8 || uint64(idx-4)+size > uint64(len(stsd)) {
			continue
		}
		return stsd[idx-4 : uint64(idx-4)+size]
	}
	return nil
}
