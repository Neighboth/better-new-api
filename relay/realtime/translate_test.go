package realtime

import (
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/relaykit/dto"

	"github.com/stretchr/testify/require"
)

// unmarshalMessage reads a message of the conversation.
func unmarshalMessage(message []byte, target any) error {
	return common.Unmarshal(message, target)
}

func TestDetectDialect(t *testing.T) {
	cases := []struct {
		name    string
		message string
		want    Dialect
	}{
		{
			name:    "the events of OpenAI have the field of the type",
			message: `{"type":"session.update","session":{"modalities":["text"]}}`,
			want:    DialectOpenAI,
		},
		{
			name:    "the setup of Gemini has the field of the setup",
			message: `{"setup":{"model":"models/gemini-2.0-flash-live-001"}}`,
			want:    DialectGemini,
		},
		{
			name:    "the audio of Gemini has the field of the input",
			message: `{"realtimeInput":{"mediaChunks":[{"data":"AA==","mimeType":"audio/pcm;rate=16000"}]}}`,
			want:    DialectGemini,
		},
		{
			name:    "the text of Gemini has the field of the content",
			message: `{"clientContent":{"turnComplete":true}}`,
			want:    DialectGemini,
		},
		{
			name:    "a message without a known field has no protocol",
			message: `{"hello":"world"}`,
			want:    DialectUnknown,
		},
	}
	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			require.Equal(t, testCase.want, DetectDialect([]byte(testCase.message)))
		})
	}
}

func TestClientOpenAIUpstreamGemini(t *testing.T) {
	translator := &bridge{
		client:   DialectOpenAI,
		upstream: DialectGemini,
		model:    "gemini-2.0-flash-live-001",
		state:    &sessionState{},
	}

	events, messages, err := translator.clientToUpstream([]byte(`{
		"type":"session.update",
		"session":{
			"modalities":["text","audio"],
			"instructions":"Be brief.",
			"voice":"alloy",
			"input_audio_format":"pcm16",
			"tools":[{"type":"function","name":"weather","description":"The weather","parameters":{"type":"object"}}]
		}
	}`))
	require.NoError(t, err)
	require.Len(t, events, 1)
	require.Len(t, messages, 1)

	// The setup of Gemini is the first message of the connection.
	var setup geminiMessage
	require.NoError(t, unmarshalMessage(messages[0], &setup))
	require.NotNil(t, setup.Setup)
	require.Equal(t, "models/gemini-2.0-flash-live-001", setup.Setup.Model)
	require.Equal(t, []string{"TEXT", "AUDIO"}, setup.Setup.GenerationConfig.ResponseModalities)
	require.Equal(t, "alloy", setup.Setup.GenerationConfig.SpeechConfig.VoiceConfig.PrebuiltVoiceConfig.VoiceName)
	require.Equal(t, "Be brief.", setup.Setup.SystemInstruction.Parts[0].Text)
	require.Len(t, setup.Setup.Tools, 1)
	require.Equal(t, "weather", setup.Setup.Tools[0].FunctionDeclarations[0].Name)

	// The audio of the client goes to the input of Gemini.
	_, messages, err = translator.clientToUpstream([]byte(`{"type":"input_audio_buffer.append","audio":"AAECAw=="}`))
	require.NoError(t, err)
	require.Len(t, messages, 1)
	var input geminiMessage
	require.NoError(t, unmarshalMessage(messages[0], &input))
	require.NotNil(t, input.RealtimeInput)
	require.Len(t, input.RealtimeInput.MediaChunks, 1)
	require.Equal(t, "audio/pcm;rate=16000", input.RealtimeInput.MediaChunks[0].MimeType)
	require.NotEmpty(t, input.RealtimeInput.MediaChunks[0].Data)

	// The request of a response ends the turn of Gemini.
	_, messages, err = translator.clientToUpstream([]byte(`{"type":"response.create"}`))
	require.NoError(t, err)
	require.Len(t, messages, 1)
	var content geminiMessage
	require.NoError(t, unmarshalMessage(messages[0], &content))
	require.NotNil(t, content.ClientContent)
	require.True(t, content.ClientContent.TurnComplete)
}

func TestClientGeminiUpstreamOpenAI(t *testing.T) {
	translator := &bridge{
		client:   DialectGemini,
		upstream: DialectOpenAI,
		model:    "gpt-4o-realtime-preview",
		state:    &sessionState{},
	}

	events, messages, err := translator.clientToUpstream([]byte(`{
		"setup":{
			"model":"models/gemini-2.0-flash-live-001",
			"generationConfig":{"responseModalities":["TEXT"],"temperature":0.5},
			"systemInstruction":{"parts":[{"text":"Be brief."}]},
			"tools":[{"functionDeclarations":[{"name":"weather","description":"The weather"}]}]
		}
	}`))
	require.NoError(t, err)
	require.Len(t, events, 1)
	require.Len(t, messages, 1)

	var event dto.RealtimeEvent
	require.NoError(t, unmarshalMessage(messages[0], &event))
	require.Equal(t, dto.RealtimeEventTypeSessionUpdate, event.Type)
	require.NotNil(t, event.Session)
	require.Equal(t, []string{"text"}, event.Session.Modalities)
	require.Equal(t, "Be brief.", event.Session.Instructions)
	require.Equal(t, 0.5, event.Session.Temperature)
	require.Len(t, event.Session.Tools, 1)
	require.Equal(t, "weather", event.Session.Tools[0].Name)

	// The audio of the client goes to the buffer of OpenAI.
	_, messages, err = translator.clientToUpstream([]byte(`{
		"realtimeInput":{"mediaChunks":[{"data":"AAECAw==","mimeType":"audio/pcm;rate=16000"}]}
	}`))
	require.NoError(t, err)
	require.Len(t, messages, 1)
	require.NoError(t, unmarshalMessage(messages[0], &event))
	require.Equal(t, dto.RealtimeEventInputAudioBufferAppend, event.Type)
	require.NotEmpty(t, event.Audio)

	// The end of the turn of the client asks for a response.
	_, messages, err = translator.clientToUpstream([]byte(`{"clientContent":{"parts":[{"text":"Hello"}],"turnComplete":true}}`))
	require.NoError(t, err)
	require.Len(t, messages, 2)
	require.NoError(t, unmarshalMessage(messages[0], &event))
	require.Equal(t, dto.RealtimeEventTypeConversationCreate, event.Type)
	require.NotNil(t, event.Item)
	require.Equal(t, "input_text", event.Item.Content[0].Type)
	require.Equal(t, "Hello", event.Item.Content[0].Text)
	require.NoError(t, unmarshalMessage(messages[1], &event))
	require.Equal(t, dto.RealtimeEventTypeResponseCreate, event.Type)
}

func TestUpstreamGeminiClientOpenAI(t *testing.T) {
	translator := &bridge{
		client:   DialectOpenAI,
		upstream: DialectGemini,
		model:    "gemini-2.0-flash-live-001",
		state:    &sessionState{},
	}

	// The text and the audio of Gemini go to the client.
	events, messages, err := translator.upstreamToClient([]byte(`{
		"serverContent":{"modelTurn":{"parts":[{"text":"Merhaba"},{"inlineData":{"mimeType":"audio/pcm;rate=24000","data":"AAEC"}}]}}
	}`))
	require.NoError(t, err)
	require.Len(t, events, 2)
	require.Len(t, messages, 2)

	var event dto.RealtimeEvent
	require.NoError(t, unmarshalMessage(messages[0], &event))
	require.Equal(t, dto.RealtimeEventResponseAudioTranscriptionDelta, event.Type)
	require.Equal(t, "Merhaba", event.Delta)
	require.NoError(t, unmarshalMessage(messages[1], &event))
	require.Equal(t, dto.RealtimeEventResponseAudioDelta, event.Type)
	require.Equal(t, "AAEC", event.Delta)

	// The end of the turn of Gemini waits for the usage of it.
	_, messages, err = translator.upstreamToClient([]byte(`{"serverContent":{"turnComplete":true}}`))
	require.NoError(t, err)
	require.Empty(t, messages)

	// The usage of Gemini completes the end of the turn.
	_, messages, err = translator.upstreamToClient([]byte(`{"usageMetadata":{"promptTokenCount":10,"responseTokenCount":5,"totalTokenCount":15}}`))
	require.NoError(t, err)
	require.Len(t, messages, 1)
	require.NoError(t, unmarshalMessage(messages[0], &event))
	require.Equal(t, dto.RealtimeEventTypeResponseDone, event.Type)
	require.NotNil(t, event.Response)
	require.NotNil(t, event.Response.Usage)
	require.Equal(t, 15, event.Response.Usage.TotalTokens)
	require.Equal(t, 10, event.Response.Usage.InputTokens)
	require.Equal(t, 5, event.Response.Usage.OutputTokens)
}

func TestUpstreamOpenAIClientGemini(t *testing.T) {
	translator := &bridge{
		client:   DialectGemini,
		upstream: DialectOpenAI,
		model:    "gpt-4o-realtime-preview",
		state:    &sessionState{},
	}

	_, messages, err := translator.upstreamToClient([]byte(`{"type":"response.audio.delta","delta":"AAEC"}`))
	require.NoError(t, err)
	require.Len(t, messages, 1)
	var content geminiMessage
	require.NoError(t, unmarshalMessage(messages[0], &content))
	require.NotNil(t, content.ServerContent)
	require.NotNil(t, content.ServerContent.ModelTurn)
	require.Equal(t, "AAEC", content.ServerContent.ModelTurn.Parts[0].InlineData.Data)

	_, messages, err = translator.upstreamToClient([]byte(`{"type":"response.audio_transcript.delta","delta":"Merhaba"}`))
	require.NoError(t, err)
	require.Len(t, messages, 1)
	require.NoError(t, unmarshalMessage(messages[0], &content))
	require.Equal(t, "Merhaba", content.ServerContent.ModelTurn.Parts[0].Text)

	_, messages, err = translator.upstreamToClient([]byte(`{
		"type":"response.function_call_arguments.done",
		"call_id":"call-1",
		"name":"weather",
		"arguments":"{\"city\":\"Istanbul\"}"
	}`))
	require.NoError(t, err)
	require.Len(t, messages, 1)
	require.NoError(t, unmarshalMessage(messages[0], &content))
	require.NotNil(t, content.ToolCall)
	require.Len(t, content.ToolCall.FunctionCalls, 1)
	require.Equal(t, "weather", content.ToolCall.FunctionCalls[0].Name)
}

func TestSameProtocolIsNotTranslated(t *testing.T) {
	translator := &bridge{
		client:   DialectOpenAI,
		upstream: DialectOpenAI,
		state:    &sessionState{},
	}
	message := []byte(`{"type":"response.create"}`)
	events, messages, err := translator.clientToUpstream(message)
	require.NoError(t, err)
	require.Equal(t, [][]byte{message}, events)
	require.Equal(t, [][]byte{message}, messages)
}
