package cloudflare

import "github.com/QuantumNous/new-api/relaykit/dto"

type CfRequest struct {
	Messages    []dto.Message `json:"messages,omitempty"`
	Lora        string        `json:"lora,omitempty"`
	MaxTokens   uint          `json:"max_tokens,omitempty"`
	Prompt      string        `json:"prompt,omitempty"`
	Raw         bool          `json:"raw,omitempty"`
	Stream      bool          `json:"stream,omitempty"`
	Temperature *float64      `json:"temperature,omitempty"`
}

type CfAudioResponse struct {
	Result     CfSTTResult `json:"result"`
	Results    CfSTTResult `json:"results"`
	Text       string      `json:"text"`
	Transcript string      `json:"transcript"`
}

type CfSTTResult struct {
	Text       string          `json:"text"`
	Transcript string          `json:"transcript"`
	Channels   []CfSTTChannel  `json:"channels"`
	Results    *CfSTTSubResult `json:"results"`
}

type CfSTTSubResult struct {
	Text       string         `json:"text"`
	Transcript string         `json:"transcript"`
	Channels   []CfSTTChannel `json:"channels"`
}

type CfSTTChannel struct {
	Alternatives []CfSTTAlternative `json:"alternatives"`
}

type CfSTTAlternative struct {
	Transcript string  `json:"transcript"`
	Confidence float64 `json:"confidence"`
}
