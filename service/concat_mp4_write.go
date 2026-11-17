/*
concat_mp4_write.go — output assembly for ConcatMP4Files.

Everything about the output is deterministic given the input sample tables, so
the moov is built twice: once with placeholder stco values to learn its size,
once with real offsets (which never changes the moov size).
*/
package service

import (
	"errors"
	"fmt"
	"os"
)

type mergedTrack struct {
	isVideo   bool
	id        uint32
	timescale uint32
	duration  uint64 // sum of part durations in track timescale

	sampleSizes []uint32
	sampleDelta []uint32
	sampleComp  []int64 // nil when every part has no ctts
	hasCTS      bool
	syncSamples []uint32 // rebased 1-based; nil when the parts have no stss

	// byte size of the payload of each part, indexed by part
	payloadPerPart []uint64

	stsdBytes []byte
	hdlrBytes []byte
	vmhdBytes []byte
	smhdBytes []byte
	dinfBytes []byte
	tkhdBytes []byte // part0 tkhd, duration patched per output

	samplesPerPart []uint32
	chunksPerPart  []uint32

	mvhdTS         uint32
	mvhdCreation   uint32
	mvhdModTime    uint32
	mvhdMatrix     []byte
	nextTrackID    uint32
	movieDuration  uint64 // summed part mvhd durations (movie timescale)
	movieTimescale uint32
	tkhdDurationMS uint32 // per-track duration in movie timescale
	videoWidth     uint32
	videoHeight    uint32
}

func writeConcat(parts []*sourcePart, outPath string) error {
	trackCount := len(parts[0].tracks)

	// Merge sample tables per track.
	merged := make([]*mergedTrack, trackCount)
	for ti := range parts[0].tracks {
		m := &mergedTrack{
			isVideo:        parts[0].tracks[ti].isVideo,
			id:             parts[0].tracks[ti].id,
			timescale:      parts[0].tracks[ti].timescale,
			stsdBytes:      parts[0].tracks[ti].stsdBytes,
			hdlrBytes:      parts[0].tracks[ti].hdlrBytes,
			vmhdBytes:      parts[0].tracks[ti].vmhdBytes,
			smhdBytes:      parts[0].tracks[ti].smhdBytes,
			dinfBytes:      parts[0].tracks[ti].dinfBytes,
			tkhdBytes:      parts[0].tracks[ti].tkhdBytes,
			videoWidth:     parts[0].tracks[ti].videoWidth,
			videoHeight:    parts[0].tracks[ti].videoHeight,
			mvhdTS:         parts[0].tracks[ti].mvhdTimescale,
			mvhdCreation:   parts[0].tracks[ti].mvhdCreation,
			mvhdModTime:    parts[0].tracks[ti].mvhdModTime,
			mvhdMatrix:     parts[0].tracks[ti].mvhdMatrix,
			movieTimescale: parts[0].tracks[ti].mvhdTimescale,
		}
		for pi, p := range parts {
			t := p.tracks[ti]
			base := uint32(0)
			for pj := 0; pj < pi; pj++ {
				base += uint32(len(parts[pj].tracks[ti].sampleSizes))
			}
			m.sampleSizes = append(m.sampleSizes, t.sampleSizes...)
			m.sampleDelta = append(m.sampleDelta, t.sampleDelta...)
			m.duration += t.duration
			m.movieDuration += t.mvhdDuration
			m.samplesPerPart = append(m.samplesPerPart, uint32(len(t.sampleSizes)))
			movieDur := (t.duration * uint64(m.movieTimescale)) / uint64(t.timescale)
			m.tkhdDurationMS += uint32(movieDur)
			if t.sampleComp != nil {
				if m.sampleComp == nil {
					// backfill zeros for the parts before this one
					need := 0
					for pj := 0; pj < pi; pj++ {
						need += len(parts[pj].tracks[ti].sampleSizes)
					}
					m.sampleComp = make([]int64, 0, need+len(t.sampleComp))
					for j := 0; j < need; j++ {
						m.sampleComp = append(m.sampleComp, 0)
					}
				}
				m.sampleComp = append(m.sampleComp, t.sampleComp...)
				m.hasCTS = true
			} else if m.sampleComp != nil {
				for j := 0; j < len(t.sampleSizes); j++ {
					m.sampleComp = append(m.sampleComp, 0)
				}
			}
			if t.syncSamples != nil {
				for _, n := range t.syncSamples {
					m.syncSamples = append(m.syncSamples, base+n)
				}
			}
			chunkSize := uint64(0)
			samplePos := 0
			for ci := range t.chunks {
				csize := uint64(0)
				for si := uint32(0); si < t.samplesPerChunk[ci]; si++ {
					csize += uint64(t.sampleSizes[samplePos])
					samplePos++
				}
				chunkSize += csize
			}
			m.payloadPerPart = append(m.payloadPerPart, chunkSize)
			m.chunksPerPart = append(m.chunksPerPart, uint32(len(t.chunks)))
		}
		if m.movieDuration > 0xffffffff {
			m.movieDuration = 0xffffffff
		}
		m.movieDuration = uint64(m.movieDuration)
		merged[ti] = m
	}

	// Negative composition offsets are not representable in an stts/ctts v0 box.
	for ti := range merged {
		for _, v := range merged[ti].sampleComp {
			if v < 0 || v > 0xffffffff {
				return fmt.Errorf("ConcatMP4Files: composition offset out of range at track %d", ti)
			}
		}
	}

	nextTrackID := uint32(1)
	for ti := range merged {
		if merged[ti].id >= nextTrackID {
			nextTrackID = merged[ti].id + 1
		}
		merged[ti].nextTrackID = 0 // set uniformly below
	}
	for ti := range merged {
		merged[ti].nextTrackID = nextTrackID
	}

	// ftyp from the first part
	ftypBox, ok := locateBox(parts[0].top, "ftyp")
	if !ok {
		return errors.New("ConcatMP4Files: ftyp box not found")
	}
	ftyp := boxBytes(parts[0].data, ftypBox)

	payloadTotal := uint64(0)
	for ti := range merged {
		for pi := range parts {
			payloadTotal += merged[ti].payloadPerPart[pi]
		}
	}
	if payloadTotal > 0xffffffff {
		return fmt.Errorf("ConcatMP4Files: merged media is %d bytes, limit is 4 GiB", payloadTotal)
	}

	moov := buildMoov(merged, nil)
	mdatStart := uint64(len(ftyp)) + uint64(len(moov)) // mdat box offset

	// Compute stco offsets per part per track.
	stco := make([][]uint64, trackCount)
	current := mdatStart + boxHeaderSize // start of mdat payload
	for ti := range merged {
		for _, psize := range merged[ti].payloadPerPart {
			stco[ti] = append(stco[ti], current)
			current += psize
		}
	}

	moov = buildMoov(merged, stco)

	// Write the output file.
	file, err := os.Create(outPath)
	if err != nil {
		return err
	}
	cleanup := func() {
		_ = file.Close()
		_ = os.Remove(outPath)
	}
	if _, err := file.Write(ftyp); err != nil {
		cleanup()
		return err
	}
	if _, err := file.Write(moov); err != nil {
		cleanup()
		return err
	}
	mdatSize := boxHeaderSize + payloadTotal
	mdatHeader := make([]byte, boxHeaderSize)
	writeUint32(mdatHeader, uint32(mdatSize))
	copy(mdatHeader[4:], "mdat")
	if _, err := file.Write(mdatHeader); err != nil {
		cleanup()
		return err
	}
	for ti := range merged {
		for pi := range parts {
			if err := writeTrackPayload(file, parts[pi], ti); err != nil {
				cleanup()
				return err
			}
		}
	}
	if err := file.Sync(); err != nil {
		cleanup()
		return err
	}
	return file.Close()
}

// writeTrackPayload streams the track payload of one part into out.
func writeTrackPayload(out *os.File, part *sourcePart, ti int) error {
	t := part.tracks[ti]
	data := part.data
	samplePos := 0
	chunkSizes := make([]uint64, 0, len(t.chunks))
	for ci := range t.chunks {
		csize := uint64(0)
		for si := uint32(0); si < t.samplesPerChunk[ci]; si++ {
			csize += uint64(t.sampleSizes[samplePos])
			samplePos++
		}
		chunkSizes = append(chunkSizes, csize)
	}
	for ci := range t.chunks {
		offset := t.chunks[ci]
		size := chunkSizes[ci]
		if offset+size > uint64(len(data)) {
			return errors.New("ConcatMP4Files: chunk out of range")
		}
		chunk := data[offset : offset+size]
		if _, err := out.Write(chunk); err != nil {
			return err
		}
	}
	return nil
}
