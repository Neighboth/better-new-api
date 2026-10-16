package deepseekweb

import (
	"bufio"
	"encoding/json"
	"io"
	"strconv"
	"strings"

	"github.com/QuantumNous/new-api/common"
)

const (
	fragmentKindThink    = "THINK"
	fragmentKindThinking = "THINKING"

	pathResponse        = "response"
	pathFragments       = "response/fragments"
	pathFragmentContent = "response/fragments/"
	pathThinkingContent = "response/thinking_content"
	pathStatus          = "response/status"

	opAppend = "APPEND"
	opSet    = "SET"
	opBatch  = "BATCH"

	statusFinished = "FINISHED"
)

// streamDelta is one increment of the assistant message produced by DeepSeek Web.
type streamDelta struct {
	content   string
	reasoning string
}

// completionStreamState turns DeepSeek Web's JSON-patch SSE protocol into plain
// deltas while accumulating the final message and the upstream token usage.
type completionStreamState struct {
	fragments []streamFragment
	usage     int
	finished  bool
}

type streamFragment struct {
	kind    string
	content string
}

// apply consumes one `data:` payload of the completion stream.
func (s *completionStreamState) apply(payload []byte) ([]streamDelta, error) {
	var patch struct {
		Path  string          `json:"p"`
		Op    string          `json:"o"`
		Value json.RawMessage `json:"v"`
	}
	if err := common.Unmarshal(payload, &patch); err != nil {
		return nil, err
	}

	if patch.Path == "" {
		return s.applyShorthand(patch.Value)
	}
	return s.applyPatch(patch.Path, patch.Op, patch.Value)
}

// applyShorthand handles the two payloads that carry no explicit path: the
// initial response snapshot and the per-token `{"v":"..."}` append.
func (s *completionStreamState) applyShorthand(value json.RawMessage) ([]streamDelta, error) {
	if len(value) == 0 {
		return nil, nil
	}
	if value[0] == '"' {
		var text string
		if err := common.Unmarshal(value, &text); err != nil {
			return nil, err
		}
		return s.appendToLastFragment(text), nil
	}

	var snapshot struct {
		Response struct {
			Fragments []struct {
				Type    string `json:"type"`
				Content string `json:"content"`
			} `json:"fragments"`
		} `json:"response"`
	}
	if err := common.Unmarshal(value, &snapshot); err != nil {
		return nil, err
	}
	if len(snapshot.Response.Fragments) == 0 || len(s.fragments) > 0 {
		return nil, nil
	}

	deltas := make([]streamDelta, 0, len(snapshot.Response.Fragments))
	for _, fragment := range snapshot.Response.Fragments {
		s.fragments = append(s.fragments, streamFragment{kind: fragment.Type, content: fragment.Content})
		if fragment.Content != "" {
			deltas = append(deltas, deltaForKind(fragment.Type, fragment.Content))
		}
	}
	return deltas, nil
}

func (s *completionStreamState) applyPatch(path, op string, value json.RawMessage) ([]streamDelta, error) {
	switch {
	case path == pathResponse && op == opBatch:
		return nil, s.applyBatch(value)
	case path == pathStatus:
		var status string
		if err := common.Unmarshal(value, &status); err != nil {
			return nil, err
		}
		if status == statusFinished {
			s.finished = true
		}
		return nil, nil
	case path == pathFragments:
		return s.appendFragments(value)
	case path == pathThinkingContent:
		return s.updateThinkingContent(op, value)
	case strings.HasPrefix(path, pathFragmentContent) && strings.HasSuffix(path, "/content"):
		return s.updateFragmentContent(path, op, value)
	}
	return nil, nil
}

// updateThinkingContent handles the legacy transport that streams reasoning
// through a dedicated field instead of a THINK fragment.
func (s *completionStreamState) updateThinkingContent(op string, value json.RawMessage) ([]streamDelta, error) {
	var text string
	if err := common.Unmarshal(value, &text); err != nil {
		return nil, err
	}
	if text == "" {
		return nil, nil
	}

	last := len(s.fragments) - 1
	if last < 0 || !isThinkFragment(s.fragments[last].kind) {
		s.fragments = append(s.fragments, streamFragment{kind: fragmentKindThink})
		last = len(s.fragments) - 1
	}
	return applyContentPatch(&s.fragments[last], op, text), nil
}

// applyContentPatch folds one content patch into a fragment and returns the new
// delta. A SET op carries the whole fragment, so only the unseen suffix is new.
func applyContentPatch(fragment *streamFragment, op, text string) []streamDelta {
	delta := text
	switch {
	case op == opAppend:
		fragment.content += text
	case strings.HasPrefix(text, fragment.content):
		delta = text[len(fragment.content):]
		fragment.content = text
	default:
		fragment.content = text
	}

	if delta == "" {
		return nil
	}
	return []streamDelta{deltaForKind(fragment.kind, delta)}
}

func (s *completionStreamState) applyBatch(value json.RawMessage) error {
	var updates []struct {
		Path  string          `json:"p"`
		Value json.RawMessage `json:"v"`
	}
	if err := common.Unmarshal(value, &updates); err != nil {
		return err
	}
	for _, update := range updates {
		switch update.Path {
		case "accumulated_token_usage":
			var usage int
			if err := common.Unmarshal(update.Value, &usage); err == nil {
				s.usage = usage
			}
		case "quasi_status":
			var status string
			if err := common.Unmarshal(update.Value, &status); err == nil && status == statusFinished {
				s.finished = true
			}
		}
	}
	return nil
}

func (s *completionStreamState) appendFragments(value json.RawMessage) ([]streamDelta, error) {
	var fragments []struct {
		Type    string `json:"type"`
		Content string `json:"content"`
	}
	if err := common.Unmarshal(value, &fragments); err != nil {
		return nil, err
	}

	deltas := make([]streamDelta, 0, len(fragments))
	for _, fragment := range fragments {
		s.fragments = append(s.fragments, streamFragment{kind: fragment.Type, content: fragment.Content})
		if fragment.Content != "" {
			deltas = append(deltas, deltaForKind(fragment.Type, fragment.Content))
		}
	}
	return deltas, nil
}

func (s *completionStreamState) updateFragmentContent(path, op string, value json.RawMessage) ([]streamDelta, error) {
	var text string
	if err := common.Unmarshal(value, &text); err != nil {
		return nil, err
	}

	index, err := strconv.Atoi(strings.Split(path, "/")[2])
	if err != nil {
		return nil, nil
	}
	if index < 0 {
		index += len(s.fragments)
	}
	if index < 0 || index >= len(s.fragments) {
		return nil, nil
	}

	return applyContentPatch(&s.fragments[index], op, text), nil
}

func (s *completionStreamState) appendToLastFragment(text string) []streamDelta {
	if text == "" || len(s.fragments) == 0 {
		return nil
	}
	last := &s.fragments[len(s.fragments)-1]
	last.content += text
	return []streamDelta{deltaForKind(last.kind, text)}
}

// content returns the assistant text accumulated so far.
func (s *completionStreamState) content() string {
	var builder strings.Builder
	for _, fragment := range s.fragments {
		if !isThinkFragment(fragment.kind) {
			builder.WriteString(fragment.content)
		}
	}
	return builder.String()
}

// reasoning returns the accumulated thinking text.
func (s *completionStreamState) reasoning() string {
	var builder strings.Builder
	for _, fragment := range s.fragments {
		if isThinkFragment(fragment.kind) {
			builder.WriteString(fragment.content)
		}
	}
	return builder.String()
}

func deltaForKind(kind, text string) streamDelta {
	if isThinkFragment(kind) {
		return streamDelta{reasoning: text}
	}
	return streamDelta{content: text}
}

func isThinkFragment(kind string) bool {
	upper := strings.ToUpper(kind)
	return upper == fragmentKindThink || upper == fragmentKindThinking
}

// readCompletionStream walks the SSE body and invokes onDelta for every token,
// stopping once upstream reports FINISHED or the stream closes.
func readCompletionStream(body io.Reader, onDelta func(streamDelta)) (*completionStreamState, error) {
	reader := bufio.NewReader(body)
	state := &completionStreamState{}

	for {
		line, err := reader.ReadString('\n')
		if trimmed := strings.TrimSpace(line); trimmed != "" {
			if payload, ok := strings.CutPrefix(trimmed, "data:"); ok {
				payload = strings.TrimSpace(payload)
				if payload != "" && payload != "[DONE]" {
					deltas, decodeErr := state.apply([]byte(payload))
					if decodeErr != nil {
						return nil, decodeErr
					}
					for _, delta := range deltas {
						onDelta(delta)
					}
				}
				if state.finished {
					return state, nil
				}
			}
		}
		if err != nil {
			if err == io.EOF {
				return state, nil
			}
			return nil, err
		}
	}
}
