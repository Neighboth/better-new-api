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
)

func TestValidMergedMediaName(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name  string
		input string
		want  bool
	}{
		{"valid merged webm", "42_1700000000000000000_abcdefgh.webm", true},
		{"valid merged mp4", "42_1700000000000000001_abcdefgh.mp4", true},
		{"empty name", "", false},
		{"path traversal", "../42_1700000000000000002_abcdefgh.mp4", false},
		{"nested path", "uploads/42_1700000000000000003_abcdefgh.mp4", false},
		{"hidden file", "42_1700000000000000004_.webm", false},
		{"missing user id", "1700000000000000005_abcdefgh.webm", false},
		{"missing random segment", "42_1700000000000000006_.webm", false},
		{"missing extension", "42_1700000000000000007_abcdefgh", false},
		{"unsupported extension", "42_1700000000000000008_abcdefgh.mkv", false},
		{"non-numeric owner", "abc_1700000000000000009_abcdefgh.webm", false},
		{"dot segments", "42_1700000000000000010_ab..cd.webm", false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			assert.Equal(t, tc.want, validMergedMediaName(tc.input))
		})
	}
}
