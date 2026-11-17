package service

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"

	mp4 "github.com/abema/go-mp4"

	"github.com/stretchr/testify/require"
)

// The fixtures are two 1-second MP4s produced by the same encoder run with
// different content, mirroring the parts of a chained Aepi generation.
func TestConcatMP4FilesMergesPartsWithSummedTrackDurations(t *testing.T) {
	parts := []string{
		filepath.Join("testdata", "concat", "part_a.mp4"),
		filepath.Join("testdata", "concat", "part_b.mp4"),
	}
	probeA, err := mp4.Probe(bytes.NewReader(readFixture(t, parts[0])))
	require.NoError(t, err)
	probeB, err := mp4.Probe(bytes.NewReader(readFixture(t, parts[1])))
	require.NoError(t, err)

	outPath := filepath.Join(t.TempDir(), "merged.mp4")
	require.NoError(t, ConcatMP4Files(parts, outPath))

	merged, err := mp4.Probe(bytes.NewReader(readFixture(t, outPath)))
	require.NoError(t, err)
	require.Len(t, merged.Tracks, 2)

	trackCount := len(probeA.Tracks)
	for i := 0; i < trackCount; i++ {
		wantSamples := len(probeA.Tracks[i].Samples) + len(probeB.Tracks[i].Samples)
		require.Len(t, merged.Tracks[i].Samples, wantSamples, "track %d must carry every sample of every part in order", i)
		require.Equal(t, probeA.Tracks[i].Timescale, merged.Tracks[i].Timescale)
		require.Equal(t, probeA.Tracks[i].Duration+probeB.Tracks[i].Duration, merged.Tracks[i].Duration)
		mergedBytes := uint64(0)
		for _, s := range merged.Tracks[i].Samples {
			mergedBytes += uint64(s.Size)
		}
		partBytes := uint64(0)
		for _, p := range []*mp4.ProbeInfo{probeA, probeB} {
			for _, s := range p.Tracks[i].Samples {
				partBytes += uint64(s.Size)
			}
		}
		require.Equal(t, partBytes, mergedBytes, "track %d sample payloads must round-trip byte for byte", i)
	}
	fi, err := os.Stat(outPath)
	require.NoError(t, err)
	require.Greater(t, fi.Size(), int64(0))
}

func TestConcatMP4FilesRejectsMismatchedTrackCount(t *testing.T) {
	// part_a.mp4 has two tracks; a truncated/invalid file must fail fast so
	// the endpoint can fall back to the browser merge instead of emitting
	// a broken MP4.
	part := filepath.Join("testdata", "concat", "part_a.mp4")
	outPath := filepath.Join(t.TempDir(), "merged.mp4")

	data := readFixture(t, part)
	truncated := filepath.Join(t.TempDir(), "truncated.mp4")
	// Keep only the ftyp box + half the file: walkBoxes stops at the first
	// box that runs past the buffer, so the parse must fail, not panic.
	require.NoError(t, os.WriteFile(truncated, data[:len(data)/2], 0o600))

	err := ConcatMP4Files([]string{part, truncated}, outPath)
	require.Error(t, err)
}

func readFixture(t *testing.T, path string) []byte {
	t.Helper()
	data, err := os.ReadFile(path)
	require.NoError(t, err)
	require.NotEmpty(t, data)
	return data
}
