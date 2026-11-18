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

package openai

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	relaycommon "github.com/QuantumNous/new-api/relay/common"
)

func TestGetRequestURLToleratesV1Suffix(t *testing.T) {
	cases := []struct {
		name string
		base string
		path string
		want string
	}{
		{"no suffix", "https://upstream.example.com", "/v1/chat/completions", "https://upstream.example.com/v1/chat/completions"},
		{"v1 suffix", "https://upstream.example.com/v1", "/v1/chat/completions", "https://upstream.example.com/v1/chat/completions"},
		{"v1 suffix with slash", "https://upstream.example.com/v1/", "/v1/chat/completions", "https://upstream.example.com/v1/chat/completions"},
		{"double v1", "https://upstream.example.com/v1/v1", "/v1/audio/transcriptions", "https://upstream.example.com/v1/audio/transcriptions"},
		{"v1 uppercase", "https://upstream.example.com/V1", "/v1/chat/completions", "https://upstream.example.com/v1/chat/completions"},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			info := &relaycommon.RelayInfo{
				ChannelMeta:    &relaycommon.ChannelMeta{},
				RequestURLPath: tc.path,
			}
			info.ChannelBaseUrl = tc.base
			url, err := (&Adaptor{}).GetRequestURL(info)
			require.NoError(t, err)
			assert.Equal(t, tc.want, url)
		})
	}
}
