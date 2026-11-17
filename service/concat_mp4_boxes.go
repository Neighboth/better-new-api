/*
concat_mp4_boxes.go — moov assembly for ConcatMP4Files.

All boxes are v0 (the caller has already established that a 32-bit mdat
offset is enough; files bigger than 4 GiB are rejected). Every box size is
deterministic given the merged sample tables, which is why the first build
pass can use placeholder stco entries.
*/
package service

func appendU16(b []byte, v uint16) []byte {
	var buf [2]byte
	writeUint16(buf[:], v)
	return append(b, buf[:]...)
}

func appendU32(b []byte, v uint32) []byte {
	var buf [4]byte
	writeUint32(buf[:], v)
	return append(b, buf[:]...)
}

func box(typ string, payload []byte) []byte {
	out := make([]byte, boxHeaderSize+len(payload))
	writeUint32(out[0:4], uint32(len(out)))
	copy(out[4:8], typ)
	copy(out[8:], payload)
	return out
}

func fullBox(typ string, payload []byte) []byte {
	full := make([]byte, 4+len(payload))
	copy(full[4:], payload)
	return box(typ, full)
}

func buildMoov(merged []*mergedTrack, stco [][]uint64) []byte {
	first := merged[0]

	// ---- mvhd ----
	mvhdPayload := []byte{}
	mvhdPayload = appendU32(mvhdPayload, 0) // version/flags
	mvhdPayload = appendU32(mvhdPayload, first.mvhdCreation)
	mvhdPayload = appendU32(mvhdPayload, first.mvhdModTime)
	mvhdPayload = appendU32(mvhdPayload, first.movieTimescale)
	mvhdPayload = appendU32(mvhdPayload, uint32(clampUint64(first.movieDuration, 0xffffffff)))
	mvhdPayload = appendU32(mvhdPayload, 0x00010000) // rate 1.0
	mvhdPayload = appendU16(mvhdPayload, 0x0100)     // volume 1.0
	mvhdPayload = appendU16(mvhdPayload, 0)          // reserved
	mvhdPayload = append(mvhdPayload, make([]byte, 8)...)
	mvhdPayload = append(mvhdPayload, first.mvhdMatrix...)
	mvhdPayload = append(mvhdPayload, make([]byte, 24)...)
	mvhdPayload = appendU32(mvhdPayload, first.nextTrackID)
	mvhd := box("mvhd", mvhdPayload)

	moovPayload := append([]byte{}, mvhd...)
	for i := range merged {
		offsets := make([]uint64, len(merged[i].payloadPerPart))
		if stco != nil {
			offsets = stco[i]
		}
		moovPayload = append(moovPayload, buildTrak(merged[i], offsets)...)
	}
	return box("moov", moovPayload)
}

func clampUint64(v uint64, max uint64) uint64 {
	if v > max {
		return max
	}
	return v
}

// buildTrak assembles one track. Callers must have validated composition
// offsets to be non-negative already (an stts/ctts v0 box cannot encode them).
func buildTrak(m *mergedTrack, offsets []uint64) []byte {
	// ---- tkhd: reuse the part0 template, patch the movie-timescale duration ----
	tkhd := append([]byte{}, m.tkhdBytes...)
	writeUint32(tkhd[28:32], m.tkhdDurationMS)

	// ---- mdhd ----
	mdhdPayload := []byte{}
	mdhdPayload = appendU32(mdhdPayload, 0) // version/flags
	mdhdPayload = appendU32(mdhdPayload, 0) // creation
	mdhdPayload = appendU32(mdhdPayload, 0) // modification
	mdhdPayload = appendU32(mdhdPayload, m.timescale)
	mdhdPayload = appendU32(mdhdPayload, uint32(clampUint64(m.duration, 0xffffffff)))
	mdhdPayload = appendU16(mdhdPayload, 0x55c4) // language: und
	mdhdPayload = appendU16(mdhdPayload, 0)      // pre_defined
	mdhd := box("mdhd", mdhdPayload)

	mdiaChildren := append(append([]byte{}, mdhd...), m.hdlrBytes...)

	// ---- minf ----
	mediaHeader := m.vmhdBytes
	if !m.isVideo {
		mediaHeader = m.smhdBytes
	}
	minfChildren := append(append([]byte{}, mediaHeader...), m.dinfBytes...)

	// ---- stbl ----
	stblPayload := append([]byte{}, m.stsdBytes...)
	stblPayload = append(stblPayload, buildStsz(m)...)
	stblPayload = append(stblPayload, buildStts(m)...)
	if m.sampleComp != nil {
		stblPayload = append(stblPayload, buildCtts(m)...)
	}
	stblPayload = append(stblPayload, buildStsc(m)...)
	stblPayload = append(stblPayload, buildStco(m, offsets)...)
	if len(m.syncSamples) > 0 {
		stblPayload = append(stblPayload, buildStss(m)...)
	}
	minfChildren = append(minfChildren, box("stbl", stblPayload)...)

	mdiaChildren = append(mdiaChildren, box("minf", minfChildren)...)
	mdia := box("mdia", mdiaChildren)
	return box("trak", append(tkhd, mdia...))
}

func buildStsz(m *mergedTrack) []byte {
	content := []byte{}
	content = appendU32(content, 0) // version/flags
	content = appendU32(content, 0) // sample_size = 0 => variable, per-sample table
	content = appendU32(content, uint32(len(m.sampleSizes)))
	for _, sz := range m.sampleSizes {
		content = appendU32(content, sz)
	}
	return box("stsz", content)
}

func rleUint32Delta(deltas []uint32) [][2]uint32 {
	runs := [][2]uint32{}
	for _, d := range deltas {
		n := len(runs)
		if n > 0 && runs[n-1][0] == d {
			runs[n-1][1]++
			continue
		}
		runs = append(runs, [2]uint32{d, 1})
	}
	return runs
}

func buildStts(m *mergedTrack) []byte {
	content := []byte{}
	content = appendU32(content, 0) // version/flags
	runs := rleUint32Delta(m.sampleDelta)
	content = appendU32(content, uint32(len(runs)))
	for _, run := range runs {
		content = appendU32(content, run[1]) // sample_count
		content = appendU32(content, run[0]) // sample_delta
	}
	return box("stts", content)
}

// buildCtts encodes per-sample composition offsets. Offsets must be
// non-negative (validated by the caller) and fit into 32 bits.
func buildCtts(m *mergedTrack) []byte {
	content := []byte{}
	content = appendU32(content, 0) // version/flags
	runs := [][2]uint32{}
	for _, off := range m.sampleComp {
		v := uint32(off)
		n := len(runs)
		if n > 0 && runs[n-1][0] == v {
			runs[n-1][1]++
			continue
		}
		runs = append(runs, [2]uint32{v, 1})
	}
	content = appendU32(content, uint32(len(runs)))
	for _, run := range runs {
		content = appendU32(content, run[1]) // sample_count
		content = appendU32(content, run[0]) // sample_composition_offset
	}
	return box("ctts", content)
}

func buildStsc(m *mergedTrack) []byte {
	content := []byte{}
	content = appendU32(content, 0) // version/flags
	entries := make([][3]uint32, 0, len(m.samplesPerPart))
	for p, samplesInPart := range m.samplesPerPart {
		entries = append(entries, [3]uint32{uint32(p + 1), samplesInPart, 1})
	}
	content = appendU32(content, uint32(len(entries)))
	for _, e := range entries {
		content = appendU32(content, e[0]) // first_chunk (1-based)
		content = appendU32(content, e[1]) // samples_per_chunk
		content = appendU32(content, e[2]) // sample_description_index
	}
	return box("stsc", content)
}

func buildStco(m *mergedTrack, offsets []uint64) []byte {
	content := []byte{}
	content = appendU32(content, 0) // version/flags
	count := len(offsets)
	content = appendU32(content, uint32(count))
	for _, off := range offsets {
		content = appendU32(content, uint32(clampUint64(off, 0xffffffff)))
	}
	return box("stco", content)
}

func buildStss(m *mergedTrack) []byte {
	content := []byte{}
	content = appendU32(content, 0) // version/flags
	content = appendU32(content, uint32(len(m.syncSamples)))
	for _, n := range m.syncSamples {
		content = appendU32(content, n)
	}
	return box("stss", content)
}
