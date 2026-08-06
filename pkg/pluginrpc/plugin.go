package pluginrpc

import (
	"net/rpc"

	"omo/pkg/pluginapi"

	"github.com/hashicorp/go-plugin"
)

// Plugin is the RPC-facing contract implemented by plugin subprocesses.
// UI is host-owned; plugins only return ViewData and handle actions.
type Plugin interface {
	GetMetadata() (pluginapi.PluginMetadata, error)
	Configure(ConfigureRequest) error
	GetView(ViewRequest) (ViewData, error)
	DoAction(ActionRequest) (ActionResult, error)
	Stop() error
}

// OmoPlugin is the go-plugin adapter for Plugin.
type OmoPlugin struct {
	Impl    Plugin
	Secrets pluginapi.SecretsProvider // unused; kept for API compat
}

func (p *OmoPlugin) Server(b *plugin.MuxBroker) (interface{}, error) {
	RPCLog("OmoPlugin.Server: creating RPCServer")
	return &RPCServer{Impl: p.Impl, broker: b}, nil
}

func (p *OmoPlugin) Client(b *plugin.MuxBroker, c *rpc.Client) (interface{}, error) {
	// Intentionally do NOT broker secrets here. Nested InitSecrets during
	// Dispense has deadlocked the host. Host pushes config via Configure.
	RPCLog("OmoPlugin.Client: creating RPCClient (no secrets broker)")
	return &RPCClient{client: c}, nil
}

// HostPluginMap returns the plugin map used by the host client.
func HostPluginMap(_ pluginapi.SecretsProvider) map[string]plugin.Plugin {
	return map[string]plugin.Plugin{
		PluginName: &OmoPlugin{},
	}
}

// ServePluginMap returns the plugin map used by plugin binaries in Serve().
func ServePluginMap(impl Plugin) map[string]plugin.Plugin {
	return map[string]plugin.Plugin{
		PluginName: &OmoPlugin{Impl: impl},
	}
}
