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

package ohvcloud

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	relaycommon "github.com/QuantumNous/new-api/relay/common"
)

func TestStripV1Suffixes(t *testing.T) {
	assert.Equal(t, "https://oai.endpoints.kepler.ai.cloud.ovh.net", stripV1Suffixes("https://oai.endpoints.kepler.ai.cloud.ovh.net/v1"))
	assert.Equal(t, "https://oai.endpoints.kepler.ai.cloud.ovh.net", stripV1Suffixes("https://oai.endpoints.kepler.ai.cloud.ovh.net/v1/"))
	assert.Equal(t, "https://oai.endpoints.kepler.ai.cloud.ovh.net", stripV1Suffixes("https://oai.endpoints.kepler.ai.cloud.ovh.net/v1/v1"))
	assert.Equal(t, "https://oai.endpoints.kepler.ai.cloud.ovh.net", stripV1Suffixes("https://oai.endpoints.kepler.ai.cloud.ovh.net/V1/"))
	assert.Equal(t, "https://oai.endpoints.kepler.ai.cloud.ovh.net", stripV1Suffixes("https://oai.endpoints.kepler.ai.cloud.ovh.net"))
	assert.Equal(t, "https://oai.endpoints.kepler.ai.cloud.ovh.net/v2", stripV1Suffixes("https://oai.endpoints.kepler.ai.cloud.ovh.net/v2"))
	assert.Equal(t, "https://example.com/v1-e", stripV1Suffixes("https://example.com/v1-e"))
	assert.Equal(t, "", stripV1Suffixes(" /v1 "))
}

func TestGetRequestURLNormalizesBaseUrl(t *testing.T) {
	for _, base := range []string{
		"https://oai.endpoints.kepler.ai.cloud.ovh.net/v1",
		"https://oai.endpoints.kepler.ai.cloud.ovh.net/v1/",
		"https://oai.endpoints.kepler.ai.cloud.ovh.net/v1/v1",
		"https://oai.endpoints.kepler.ai.cloud.ovh.net",
	} {
		info := &relaycommon.RelayInfo{
			ChannelMeta: &relaycommon.ChannelMeta{},
		}
		info.ChannelBaseUrl = base
		info.RequestURLPath = "/v1/audio/transcriptions"
		info.ChannelType = 69
		_, err := (&Adaptor{}).GetRequestURL(info)
		require.NoError(t, err)
		assert.Equal(t, "https://oai.endpoints.kepler.ai.cloud.ovh.net", info.ChannelBaseUrl)
	}
}
