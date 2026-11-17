package aepi

import (
	"testing"

	relaycommon "github.com/QuantumNous/new-api/relay/common"
)

// The Aepi catalog drifts: values this code clamped in the past can stop being
// valid (minimax-h3-max-turbo moved from 720p to 480p/768p on 2026-11-17).
// The request must fall back to values the model supports instead of bouncing
// off the upstream with an opaque error.
func TestNormalizeAndValidateVideoRequestClampsToCatalog(t *testing.T) {
	tests := []struct {
		name        string
		model       string
		duration    int
		ratio       string
		quality     string
		wantRatio   string
		wantQuality string
		wantDur     int
	}{
		{
			name:        "minimax-h3-max-turbo unsupported 720p asked, falls back to supported value",
			model:       "minimax-h3-max-turbo",
			duration:    20,
			ratio:       "16:9",
			quality:     "720p",
			wantRatio:   "16:9",
			wantQuality: "480p",
			wantDur:     15,
		},
		{
			name:        "seedance-2.5 keeps supported 1080p",
			model:       "seedance-2.5",
			duration:    6,
			ratio:       "16:9",
			quality:     "1080p",
			wantRatio:   "16:9",
			wantQuality: "1080p",
			wantDur:     6,
		},
		{
			name:        "seedance-2.0 unsupported 21:9 falls back to first supported aspect",
			model:       "seedance-2.0",
			duration:    4,
			ratio:       "2:1",
			quality:     "720p",
			wantRatio:   "auto",
			wantQuality: "720p",
			wantDur:     4,
		},
		{
			name:        "duration below model minimum is raised to the minimum",
			model:       "wan-2.7",
			duration:    1,
			ratio:       "1:1",
			quality:     "720p",
			wantRatio:   "1:1",
			wantQuality: "720p",
			wantDur:     2,
		},
		{
			name:        "unknown model passes through untouched",
			model:       "some-future-model",
			duration:    12,
			ratio:       "16:9",
			quality:     "1080p",
			wantRatio:   "16:9",
			wantQuality: "1080p",
			wantDur:     12,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := &relaycommon.TaskSubmitReq{
				Model:    tt.model,
				Duration: tt.duration,
				Ratio:    tt.ratio,
				Quality:  tt.quality,
			}
			got := normalizeAndValidateVideoRequest(req)
			if got != nil {
				t.Fatalf("normalizeAndValidateVideoRequest returned error: %+v", got)
			}
			if req.Ratio != tt.wantRatio {
				t.Errorf("Ratio = %q, want %q", req.Ratio, tt.wantRatio)
			}
			if req.Quality != tt.wantQuality {
				t.Errorf("Quality = %q, want %q", req.Quality, tt.wantQuality)
			}
			if req.Duration != tt.wantDur {
				t.Errorf("Duration = %d, want %d", req.Duration, tt.wantDur)
			}
		})
	}
}
