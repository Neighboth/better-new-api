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

// OHVCloud (Anon) — the public, anonymous, OpenAI-compatible gateway of
// OVHcloud's AI Endpoints (https://oai.endpoints.kepler.ai.cloud.ovh.net).
// It needs no API key and exposes a /v1/models list where every pricing
// field of the free models is "0".
//
// The configured base URL may carry the /v1 suffix exactly as the user
// pastes it (https://oai.endpoints.kepler.ai.cloud.ovh.net/v1); the adaptor
// strips it so the shared /v1/<path> relay segments never duplicate.
package ohvcloud

import (
	"strings"

	"github.com/QuantumNous/new-api/relay/channel/openai"
	relaycommon "github.com/QuantumNous/new-api/relay/common"
)

type Adaptor struct {
	openai.Adaptor
}

func (a *Adaptor) GetRequestURL(info *relaycommon.RelayInfo) (string, error) {
	if info != nil {
		base := strings.TrimSpace(info.ChannelBaseUrl)
		trimmed := strings.TrimSuffix(strings.TrimRight(base, "/"), "/v1")
		if trimmed != base {
			info.ChannelBaseUrl = trimmed
		}
	}
	return a.Adaptor.GetRequestURL(info)
}
