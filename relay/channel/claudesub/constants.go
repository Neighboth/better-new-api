package claudesub

const ChannelName = "claude_subscription"

// ModelList mirrors the models a Claude subscription (Claude Code / Claude
// Agent SDK) can reach. Operators curate this freely from the channel UI.
var ModelList = []string{
	"claude-opus-4-6",
	"claude-opus-4-7",
	"claude-opus-4-8",
	"claude-opus-5",
	"claude-opus-5-5",
	"claude-sonnet-4-5-20250929",
	"claude-sonnet-4-6",
	"claude-sonnet-5",
	"claude-sonnet-5-5",
	"claude-haiku-4-5-20251001",
	"claude-haiku-5-5",
	"claude-fable-5",
}
