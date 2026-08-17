package protocol

const Version = 1
const AgentVersion = "1.4.0"

var Capabilities = []string{
	"codex-refresh",
	"claude-refresh",
	"mdns",
	"pairing-v2",
	"device-tokens",
	"auth-subprotocol",
	"device-management",
	"master-local-only",
	"snapshot-cache",
	"token-usage",
}
