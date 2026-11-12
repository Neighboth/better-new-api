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
package aepi

import (
	"fmt"
	"net/http"
	"strings"

	relaycommon "github.com/QuantumNous/new-api/relay/common"
	"github.com/QuantumNous/new-api/service"

	taskdto "github.com/QuantumNous/new-api/dto"
)

// normalizeAndValidateVideoRequest fills the aspect and the resolution of a
// video task request from the catalog of the model and rejects the values the
// model does not support. It runs before the request is billed, so a
// misconfigured request is answered with a clear 400 instead of an opaque
// 422 halfway through the render.
//
// When the catalog of the model is known, an empty aspect or an empty
// resolution is filled with the first supported one: the provider default
// ("auto") is rejected by the Veo family, and the request used to fail with
// "Aepi generation failed: Unexpected status code: 422".
func normalizeAndValidateVideoRequest(req *relaycommon.TaskSubmitReq) *taskdto.TaskError {
	ratio := strings.TrimSpace(strings.ToLower(req.Ratio))
	quality := strings.TrimSpace(strings.ToLower(req.Quality))
	modelName := strings.ToLower(strings.TrimSpace(req.Model))

	if entry, known := videoModelConstraints[modelName]; known {
		if ratio == "" && len(entry.Aspects) > 0 {
			ratio = entry.Aspects[0]
		}
		if ratio != "" && len(entry.Aspects) > 0 && !constraintContains(entry.Aspects, ratio) {
			return service.TaskErrorWrapperLocal(
				fmt.Errorf("model %s does not support aspect %q (allowed: %s)", req.Model, ratio, strings.Join(entry.Aspects, ", ")),
				"invalid_request", http.StatusBadRequest,
			)
		}
		if quality == "" && len(entry.RenderRes) > 0 {
			quality = entry.RenderRes[0]
		}
		if quality != "" && len(entry.RenderRes) > 0 && !constraintContains(entry.RenderRes, quality) {
			return service.TaskErrorWrapperLocal(
				fmt.Errorf("model %s does not support resolution %q (allowed: %s)", req.Model, quality, strings.Join(entry.RenderRes, ", ")),
				"invalid_request", http.StatusBadRequest,
			)
		}
	}

	req.Ratio = ratio
	req.Quality = quality
	return nil
}

func constraintContains(list []string, value string) bool {
	for _, item := range list {
		if item == value {
			return true
		}
	}
	return false
}
