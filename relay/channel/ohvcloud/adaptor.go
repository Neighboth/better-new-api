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

// stripV1Suffixes removes any trailing /v1 segments (case-insensitive) from
// the base URL. The shared relay path always appends /v1/<endpoint>, so a
// base that still carries one or more /v1 suffixes would produce
// /v1/v1/<endpoint> and upstreams answer with 404.
func stripV1Suffixes(base string) string {
	trimmed := strings.TrimRight(strings.TrimSpace(base), "/")
	lower := strings.ToLower(trimmed)
	for strings.HasSuffix(lower, "/v1") {
		trimmed = strings.TrimRight(trimmed[:len(trimmed)-3], "/")
		lower = strings.ToLower(trimmed)
	}
	return trimmed
}

func (a *Adaptor) GetRequestURL(info *relaycommon.RelayInfo) (string, error) {
	if info != nil {
		if trimmed := stripV1Suffixes(info.ChannelBaseUrl); trimmed != info.ChannelBaseUrl {
			info.ChannelBaseUrl = trimmed
		}
	}
	return a.Adaptor.GetRequestURL(info)
}
