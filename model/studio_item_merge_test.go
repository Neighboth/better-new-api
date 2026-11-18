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

package model

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/QuantumNous/new-api/common"
)

func TestMergeStudioPayloadPreservesOptimizerFacts(t *testing.T) {
	stored := `{"id":"a","taskId":"TASK-1","details":{"optimizedPrompt":"OPT","negativePrompt":"NEG","referenceImages":["https://x/r1.webp"]}}`
	incoming := `{"id":"a","status":"pending","progress":"43%","settings":{"model":"seedance-2.5"},"details":{"referenceImages":["https://x/r1.webp"],"startFrame":"https://x/f.webp"}}`
	out, err := mergeStudioPayloadPreservingServerFacts(stored, incoming)
	require.NoError(t, err)
	payload := map[string]any{}
	require.NoError(t, common.UnmarshalJsonStr(out, &payload))
	assert.Equal(t, "TASK-1", payload["taskId"])
	assert.Equal(t, "43%", payload["progress"])
	assert.Equal(t, "OPT", payload["details"].(map[string]any)["optimizedPrompt"])
	assert.Equal(t, "NEG", payload["details"].(map[string]any)["negativePrompt"])
	assert.Equal(t, "https://x/f.webp", payload["details"].(map[string]any)["startFrame"])
	_ = strings.TrimSpace(out)
}

func TestMergeStudioPayloadIncomingOptimizedWins(t *testing.T) {
	stored := `{"id":"a","taskId":"TASK-1","details":{"optimizedPrompt":"OPT","negativePrompt":"NEG"}}`
	incoming := `{"id":"a","details":{"optimizedPrompt":"USER-OPT"}}`
	out, err := mergeStudioPayloadPreservingServerFacts(stored, incoming)
	require.NoError(t, err)
	payload := map[string]any{}
	require.NoError(t, common.UnmarshalJsonStr(out, &payload))
	assert.Equal(t, "TASK-1", payload["taskId"])
	assert.Equal(t, "USER-OPT", payload["details"].(map[string]any)["optimizedPrompt"])
	assert.Equal(t, "NEG", payload["details"].(map[string]any)["negativePrompt"])
}

func TestMergeStudioPayloadTaskIDComesFromServer(t *testing.T) {
	stored := `{"id":"a","taskId":"TASK-9"}`
	incoming := `{"id":"a","progress":"71%"}`
	out, err := mergeStudioPayloadPreservingServerFacts(stored, incoming)
	require.NoError(t, err)
	payload := map[string]any{}
	require.NoError(t, common.UnmarshalJsonStr(out, &payload))
	assert.Equal(t, "TASK-9", payload["taskId"])
	assert.Equal(t, "71%", payload["progress"])
}

func TestMergeStudioPayloadNoStoredFacts(t *testing.T) {
	stored := `{"id":"a"}`
	incoming := `{"id":"a","progress":"12%","details":{"optimizedPrompt":"OPT"}}`
	out, err := mergeStudioPayloadPreservingServerFacts(stored, incoming)
	require.NoError(t, err)
	payload := map[string]any{}
	require.NoError(t, common.UnmarshalJsonStr(out, &payload))
	assert.Equal(t, "OPT", payload["details"].(map[string]any)["optimizedPrompt"])
}
