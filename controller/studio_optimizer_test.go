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
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestRemoveNegativeContradictionsDropsFixedCameraConflicts(t *testing.T) {
	out := removeNegativeContradictions(
		"Fixed camera on a wet road in rain, a car passes",
		"camera shake, handheld, no rain, watermark",
	)
	assert.Equal(t, "watermark", out)
}

func TestRemoveNegativeContradictionsDropsConflictsWithRain(t *testing.T) {
	out := removeNegativeContradictions(
		"Heavy rain on the windshield, night scene",
		"no rain, dry roads, blurry",
	)
	assert.Equal(t, "blurry", out)
}

func TestRemoveNegativeContradictionsKeepsBenignClauses(t *testing.T) {
	out := removeNegativeContradictions(
		"A sunny beach",
		"watermark, logo",
	)
	assert.Equal(t, "watermark, logo", out)
}

func TestRemoveNegativeContradictionsEmptyInput(t *testing.T) {
	assert.Equal(t, "", removeNegativeContradictions("anything", ""))
}

func TestParseToolCallReadsFencedAndBareJSON(t *testing.T) {
	call := parseToolCall("```json\n{\"tool\":\"web_search\",\"arguments\":{\"query\":\"Tofaş\"}}\n```")
	require.NotNil(t, call)
	assert.Equal(t, "web_search", call.Tool)
	assert.Equal(t, "Tofaş", call.Query)

	bare := parseToolCall("{\"tool\":\"image_search\",\"arguments\":{\"query\":\"Verity\"}}")
	require.NotNil(t, bare)
	assert.Equal(t, "image_search", bare.Tool)
	assert.Equal(t, "Verity", bare.Query)

	assert.Nil(t, parseToolCall("just a prompt, not a tool call"))
}

func TestParseOptimizerModelsSplitsAndTrims(t *testing.T) {
	assert.Equal(t, []string{"gpt-4o-mini", "gemini-2.5-flash"}, parseOptimizerModels(" gpt-4o-mini , gemini-2.5-flash "))
	assert.Equal(t, []string{"a"}, parseOptimizerModels("a"))
	assert.Empty(t, parseOptimizerModels(" , "))
}
