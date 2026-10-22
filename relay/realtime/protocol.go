// Package realtime relays a realtime conversation between a client and a
// channel.
//
// The clients of the installation and the channels do not always speak the
// same protocol. The protocol of OpenAI is the common language of them, the
// messages are translated with it when the two sides do not speak the same
// protocol.
package realtime

import (
	"encoding/json"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/constant"
	relaycommon "github.com/QuantumNous/new-api/relay/common"
)

// UpstreamDialect returns the protocol of the channel of the conversation.
//
// The channels of Gemini speak the live protocol of Gemini, the other channels
// speak the realtime protocol of OpenAI.
func UpstreamDialect(info *relaycommon.RelayInfo) Dialect {
	if info != nil && info.ApiType == constant.APITypeGemini {
		return DialectGemini
	}
	return DialectOpenAI
}

// Dialect is the protocol of a realtime conversation.
type Dialect int

const (
	// DialectUnknown is the state before the first message of the client.
	DialectUnknown Dialect = iota
	// DialectOpenAI is the realtime protocol of OpenAI.
	DialectOpenAI
	// DialectGemini is the live protocol of Gemini.
	DialectGemini
)

func (d Dialect) String() string {
	switch d {
	case DialectOpenAI:
		return "openai"
	case DialectGemini:
		return "gemini"
	default:
		return "unknown"
	}
}

// DetectDialect reads the protocol of a message of a client.
//
// The clients of OpenAI put the name of the event into the field "type", the
// clients of Gemini put the name of the message into the fields "setup",
// "realtimeInput", "clientContent" and "toolResponse".
func DetectDialect(message []byte) Dialect {
	var probe map[string]json.RawMessage
	if err := common.Unmarshal(message, &probe); err != nil {
		return DialectUnknown
	}
	if _, ok := probe["type"]; ok {
		return DialectOpenAI
	}
	for _, field := range []string{"setup", "realtimeInput", "clientContent", "toolResponse", "toolCancellation"} {
		if _, ok := probe[field]; ok {
			return DialectGemini
		}
	}
	return DialectUnknown
}

// geminiMessage is a message of the live protocol of Gemini.
//
// The messages of the client and the messages of the server do not have the
// same fields, one structure holds the fields of the two of them.
type geminiMessage struct {
	Setup               *geminiSetup            `json:"setup,omitempty"`
	RealtimeInput       *geminiRealtimeInput    `json:"realtimeInput,omitempty"`
	ClientContent       *geminiClientContent    `json:"clientContent,omitempty"`
	ToolResponse        *geminiToolResponse     `json:"toolResponse,omitempty"`
	ToolCancellation    *geminiToolCancellation `json:"toolCancellation,omitempty"`
	SetupComplete       *geminiEmpty            `json:"setupComplete,omitempty"`
	ServerContent       *geminiServerContent    `json:"serverContent,omitempty"`
	ToolCall            *geminiToolCall         `json:"toolCall,omitempty"`
	UsageMetadata       *geminiUsage            `json:"usageMetadata,omitempty"`
	LiveDisconnect      *geminiEmpty            `json:"liveDisconnect,omitempty"`
	GoAway              *geminiEmpty            `json:"goAway,omitempty"`
	Error               *geminiError            `json:"error,omitempty"`
	InputTranscription  *geminiTranscription    `json:"inputTranscription,omitempty"`
	OutputTranscription *geminiTranscription    `json:"outputTranscription,omitempty"`
}

type geminiEmpty struct{}

type geminiError struct {
	Code    int    `json:"code,omitempty"`
	Message string `json:"message,omitempty"`
	Status  string `json:"status,omitempty"`
}

type geminiSetup struct {
	Model                    string                          `json:"model,omitempty"`
	GenerationConfig         *geminiGenerationConfig         `json:"generationConfig,omitempty"`
	SystemInstruction        *geminiContent                  `json:"systemInstruction,omitempty"`
	Tools                    []geminiTool                    `json:"tools,omitempty"`
	ToolConfig               *geminiToolConfig               `json:"toolConfig,omitempty"`
	RealtimeInputConfig      *geminiRealtimeInputConfig      `json:"realtimeInputConfig,omitempty"`
	InputTranscription       *geminiTranscription            `json:"inputTranscription,omitempty"`
	OutputTranscription      *geminiTranscription            `json:"outputTranscription,omitempty"`
	SessionResumption        string                          `json:"sessionResumption,omitempty"`
	ContextWindowCompression *geminiContextWindowCompression `json:"contextWindowCompression,omitempty"`
}

type geminiGenerationConfig struct {
	ResponseModalities []string            `json:"responseModalities,omitempty"`
	SpeechConfig       *geminiSpeechConfig `json:"speechConfig,omitempty"`
	Temperature        *float64            `json:"temperature,omitempty"`
	MaxOutputTokens    int                 `json:"maxOutputTokens,omitempty"`
}

type geminiSpeechConfig struct {
	VoiceConfig *geminiVoiceConfig `json:"voiceConfig,omitempty"`
}

type geminiVoiceConfig struct {
	PrebuiltVoiceConfig *geminiPrebuiltVoiceConfig `json:"prebuiltVoiceConfig,omitempty"`
}

type geminiPrebuiltVoiceConfig struct {
	VoiceName string `json:"voiceName,omitempty"`
}

type geminiContent struct {
	Parts []geminiPart `json:"parts,omitempty"`
	Role  string       `json:"role,omitempty"`
}

type geminiPart struct {
	Text       string            `json:"text,omitempty"`
	InlineData *geminiInlineData `json:"inlineData,omitempty"`
}

type geminiInlineData struct {
	MimeType string `json:"mimeType,omitempty"`
	Data     string `json:"data,omitempty"`
}

type geminiTool struct {
	FunctionDeclarations []geminiFunctionDeclaration `json:"functionDeclarations,omitempty"`
}

type geminiFunctionDeclaration struct {
	Name        string `json:"name,omitempty"`
	Description string `json:"description,omitempty"`
	Parameters  any    `json:"parameters,omitempty"`
}

type geminiToolConfig struct {
	FunctionCallingConfig *geminiFunctionCallingConfig `json:"functionCallingConfig,omitempty"`
}

type geminiFunctionCallingConfig struct {
	Mode string `json:"mode,omitempty"`
}

type geminiRealtimeInputConfig struct {
	AutomaticActivityDetection *geminiAutomaticActivityDetection `json:"automaticActivityDetection,omitempty"`
	ActivityHandling           string                            `json:"activityHandling,omitempty"`
	TurnCoverage               string                            `json:"turnCoverage,omitempty"`
}

type geminiAutomaticActivityDetection struct {
	Disabled      bool `json:"disabled,omitempty"`
	StartOfSpeech bool `json:"startOfSpeech,omitempty"`
	EndOfSpeech   bool `json:"endOfSpeech,omitempty"`
}

type geminiTranscription struct {
	Language string `json:"language,omitempty"`
}

type geminiContextWindowCompression struct {
	TriggerTokens int  `json:"triggerTokens,omitempty"`
	SlidingWindow bool `json:"slidingWindow,omitempty"`
}

type geminiRealtimeInput struct {
	MediaChunks    []geminiMediaChunk `json:"mediaChunks,omitempty"`
	AudioStreamEnd bool               `json:"audioStreamEnd,omitempty"`
	TurnComplete   bool               `json:"turnComplete,omitempty"`
}

type geminiMediaChunk struct {
	Data     string `json:"data,omitempty"`
	MimeType string `json:"mimeType,omitempty"`
}

type geminiClientContent struct {
	Parts        []geminiPart `json:"parts,omitempty"`
	TurnComplete bool         `json:"turnComplete,omitempty"`
}

type geminiToolResponse struct {
	FunctionResponses []geminiFunctionResponse `json:"functionResponses,omitempty"`
}

type geminiFunctionResponse struct {
	ID       string         `json:"id,omitempty"`
	Name     string         `json:"name,omitempty"`
	Response map[string]any `json:"response,omitempty"`
}

type geminiToolCancellation struct {
	FunctionIds []string `json:"functionIds,omitempty"`
}

type geminiServerContent struct {
	ModelTurn           *geminiContent       `json:"modelTurn,omitempty"`
	TurnComplete        bool                 `json:"turnComplete,omitempty"`
	Interrupted         bool                 `json:"interrupted,omitempty"`
	InputTranscription  *geminiTranscription `json:"inputTranscription,omitempty"`
	OutputTranscription *geminiTranscription `json:"outputTranscription,omitempty"`
}

type geminiToolCall struct {
	FunctionCalls []geminiFunctionCall `json:"functionCalls,omitempty"`
}

type geminiFunctionCall struct {
	ID   string         `json:"id,omitempty"`
	Name string         `json:"name,omitempty"`
	Args map[string]any `json:"args,omitempty"`
}

type geminiUsage struct {
	PromptTokenCount        int `json:"promptTokenCount,omitempty"`
	ResponseTokenCount      int `json:"responseTokenCount,omitempty"`
	TotalTokenCount         int `json:"totalTokenCount,omitempty"`
	ThoughtsTokenCount      int `json:"thoughtsTokenCount,omitempty"`
	CachedContentTokenCount int `json:"cachedContentTokenCount,omitempty"`
}
