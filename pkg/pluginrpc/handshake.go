package pluginrpc

import "github.com/hashicorp/go-plugin"

// Handshake is shared between the omo host and RPC plugin binaries.
// A mismatch shows a clear error instead of obscure protocol failures.
var Handshake = plugin.HandshakeConfig{
	ProtocolVersion:  1,
	MagicCookieKey:   "OMO_PLUGIN",
	MagicCookieValue: "omo-plugin",
}

// PluginName is the go-plugin dispense key for the omo plugin interface.
const PluginName = "omo"
