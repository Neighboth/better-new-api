/*
Copyright (C) 2023-2026 QuantumNous

This program is free software: you can redistribute it and/or modify
it under the terms of the GNU Affero General Public License as
published by the Free Software Foundation, either version 3 of the
License, or (at your option) any later version.

This program is distributed in the hope that it will be useful,
but WITHOUT ANY WARRANTY; without even the implied warranty of
MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE. See the
GNU Affero General Public License for more details.

You should have received a copy of the GNU Affero General Public License
along with this program. If not, see <https://www.gnu.org/licenses/>.

For commercial licensing, please contact support@quantumnous.com
*/

package controller

import (
	"fmt"
	"strings"

	"github.com/gin-gonic/gin"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/service"

	relaycommon "github.com/QuantumNous/new-api/relay/common"
)

func containsIntFold(list []int, want int) bool {
	for _, value := range list {
		if value == want {
			return true
		}
	}
	return false
}

func containsStringFold(list []string, want string) bool {
	want = strings.TrimSpace(strings.ToLower(want))
	for _, value := range list {
		if strings.ToLower(strings.TrimSpace(value)) == want {
			return true
		}
	}
	return false
}

// metadataCount reads a numeric metadata counter the client may set to
// distinguish image/audio/video references in a flat reference_images list.
func metadataCount(metadata map[string]interface{}, key string) (int, bool) {
	if metadata == nil {
		return 0, false
	}
	switch v := metadata[key].(type) {
	case int:
		if v < 0 {
			return 0, true
		}
		return v, true
	case int64:
		if v < 0 {
			return 0, true
		}
		return int(v), true
	case float64:
		if v < 0 {
			return 0, true
		}
		return int(v), true
	default:
		return 0, false
	}
}

// referenceCounts resolves the per-modality reference counts. When the client
// supplies explicit counters they win; otherwise the whole flat list counts as
// image references (conservative).
func referenceCounts(metadata map[string]interface{}, flat []string) (images, audio, video int) {
	if img, ok := metadataCount(metadata, "reference_image_count"); ok {
		aud, _ := metadataCount(metadata, "reference_audio_count")
		vid, _ := metadataCount(metadata, "reference_video_count")
		return img, aud, vid
	}
	return len(flat), 0, 0
}

func hasStudioEditSource(req *relaycommon.TaskSubmitReq) bool {
	if strings.TrimSpace(req.InputReference) != "" || strings.TrimSpace(req.Image) != "" {
		return true
	}
	if len(req.Images) > 0 && strings.TrimSpace(req.Images[0]) != "" {
		return true
	}
	for _, key := range []string{"source", "start_frame"} {
		if value := metadataStringValue(req.Metadata, key); value != "" {
			return true
		}
	}
	if len(metadataReferenceImages(req.Metadata)) > 0 {
		return true
	}
	return false
}

func denyStudioRequest(c *gin.Context, format string, args ...interface{}) bool {
	common.ApiErrorMsg(c, fmt.Sprintf(format, args...))
	return false
}

// enforceStudioVideoRules validates a video submission against the configured
// model's option rules (durations, qualities, ratios, reference caps, audio,
// edit mode). Unconfigured models pass through. Returns true when the request
// may proceed.
func enforceStudioVideoRules(c *gin.Context, req *relaycommon.TaskSubmitReq) bool {
	m := service.FindMediaModel("video", req.Model)
	if m == nil {
		return true
	}
	effective := service.ResolveEffectiveStudioModel(*m, req.Duration, req.Duration > 0, req.Quality, req.Ratio)
	if len(effective.Durations) > 0 && req.Duration > 0 && !containsIntFold(effective.Durations, req.Duration) {
		return denyStudioRequest(c, "Model %s does not support a %ds duration at these settings.", m.Model, req.Duration)
	}
	if q := strings.TrimSpace(req.Quality); q != "" && len(effective.Qualities) > 0 && !containsStringFold(effective.Qualities, q) {
		return denyStudioRequest(c, "Model %s does not support quality %q at these settings.", m.Model, q)
	}
	if r := strings.TrimSpace(req.Ratio); r != "" && len(effective.Ratios) > 0 && !containsStringFold(effective.Ratios, r) {
		return denyStudioRequest(c, "Model %s does not support aspect ratio %q at these settings.", m.Model, r)
	}

	flat := metadataReferenceImages(req.Metadata)
	images, audio, video := referenceCounts(req.Metadata, flat)

	imageCap := effective.MaxReferenceImages
	if images > imageCap {
		if !effective.ReferencesAllowed {
			return denyStudioRequest(c, "Model %s does not accept reference images at these settings.", m.Model)
		}
		return denyStudioRequest(c, "Model %s accepts at most %d reference image(s) at these settings.", m.Model, imageCap)
	}
	audioCap := 0
	if m.AudioReference {
		audioCap = m.MaxAudioReferences
		if audioCap <= 0 {
			audioCap = 3
		}
	}
	videoCap := 0
	if m.VideoReference {
		videoCap = m.MaxVideoReferences
		if videoCap <= 0 {
			videoCap = 2
		}
	}
	if audio > audioCap {
		if audioCap <= 0 {
			return denyStudioRequest(c, "Model %s does not accept audio references.", m.Model)
		}
		return denyStudioRequest(c, "Model %s accepts at most %d audio reference(s).", m.Model, audioCap)
	}
	if video > videoCap {
		if videoCap <= 0 {
			return denyStudioRequest(c, "Model %s does not accept video references.", m.Model)
		}
		return denyStudioRequest(c, "Model %s accepts at most %d video reference(s).", m.Model, videoCap)
	}

	// Audio generation: honor the model's capability, never the request.
	if raw, ok := req.Metadata["audio"]; ok {
		wantAudio, isBool := raw.(bool)
		if isBool && wantAudio && !m.Audio {
			delete(req.Metadata, "audio")
		}
	}

	// Video edit capability gates.
	operation := metadataStringValue(req.Metadata, "operation")
	if operation == "edit" {
		if m.EditMode == "" || m.EditMode == "none" {
			return denyStudioRequest(c, "Model %s does not support video editing.", m.Model)
		}
		if !hasStudioEditSource(req) {
			return denyStudioRequest(c, "Model %s edits an input video — attach one first.", m.Model)
		}
	}
	if m.EditMode == "only" && !hasStudioEditSource(req) {
		return denyStudioRequest(c, "Model %s edits an input video — attach one before generating.", m.Model)
	}
	return true
}

// enforceStudioImageRules validates an image submission against the configured
// model's option rules (sizes, qualities, reference caps). Unconfigured models
// pass through.
func enforceStudioImageRules(c *gin.Context, modelName string, size string, quality string, metadata map[string]interface{}) bool {
	m := service.FindMediaModel("image", modelName)
	if m == nil {
		return true
	}
	effective := service.ResolveEffectiveStudioModel(*m, 0, false, quality, "")
	if s := strings.TrimSpace(size); s != "" && len(m.Sizes) > 0 && !containsStringFold(m.Sizes, s) {
		return denyStudioRequest(c, "Model %s does not support size %q.", m.Model, s)
	}
	if q := strings.TrimSpace(quality); q != "" && len(effective.Qualities) > 0 && !containsStringFold(effective.Qualities, q) {
		return denyStudioRequest(c, "Model %s does not support quality %q at these settings.", m.Model, q)
	}
	flat := metadataReferenceImages(metadata)
	images, _, _ := referenceCounts(metadata, flat)
	if images > effective.MaxReferenceImages {
		if !effective.ReferencesAllowed {
			return denyStudioRequest(c, "Model %s does not accept reference images.", m.Model)
		}
		return denyStudioRequest(c, "Model %s accepts at most %d reference image(s) at these settings.", m.Model, effective.MaxReferenceImages)
	}
	return true
}
