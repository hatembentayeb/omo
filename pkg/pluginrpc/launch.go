package pluginrpc

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"time"

	"omo/pkg/pluginapi"

	"github.com/hashicorp/go-hclog"
	"github.com/hashicorp/go-plugin"
)

// Launch starts an RPC plugin binary and returns a go-plugin client plus
// the dispensed Plugin interface. Caller must Kill the client when done.
//
// Secrets are NOT brokered over MuxBroker here — the host pushes connection
// settings via Configure to avoid nested net/rpc deadlocks.
func Launch(binPath string) (*plugin.Client, Plugin, error) {
	RPCLog("Launch begin bin=%s", binPath)
	start := time.Now()

	logPath := filepath.Join(pluginapi.LogsDir(), "go-plugin-host.log")
	_ = os.MkdirAll(pluginapi.LogsDir(), 0755)
	logFile, err := os.OpenFile(logPath, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0644)
	if err != nil {
		RPCLog("Launch: cannot open go-plugin log: %v", err)
		logFile = nil
	}

	var logger hclog.Logger
	if logFile != nil {
		logger = hclog.New(&hclog.LoggerOptions{
			Name:   "omo-plugin-host",
			Output: logFile,
			Level:  hclog.Debug,
		})
	} else {
		logger = hclog.New(&hclog.LoggerOptions{
			Name:  "omo-plugin-host",
			Level: hclog.Debug,
		})
	}

	cmd := exec.Command(binPath)
	// Ensure plugin child can write its own logs under ~/.omo/logs
	cmd.Env = append(os.Environ(), "OMO_RPC_LOG=1")

	client := plugin.NewClient(&plugin.ClientConfig{
		HandshakeConfig: Handshake,
		Plugins:         HostPluginMap(nil), // no secrets broker
		Cmd:             cmd,
		AllowedProtocols: []plugin.Protocol{
			plugin.ProtocolNetRPC,
		},
		Logger: logger,
		// Reap plugin children when Kill() is called (host Shutdown / KillAll).
		Managed: true,
		// Avoid hanging forever if the plugin never handshakes.
		StartTimeout: 15 * time.Second,
	})

	RPCLog("Launch: calling client.Client() …")
	rpcClient, err := client.Client()
	if err != nil {
		client.Kill()
		RPCLog("Launch: client.Client failed after %s: %v", time.Since(start), err)
		return nil, nil, fmt.Errorf("connect to plugin %s: %w", binPath, err)
	}
	RPCLog("Launch: client.Client OK in %s", time.Since(start))

	RPCLog("Launch: Dispense(%s) …", PluginName)
	raw, err := rpcClient.Dispense(PluginName)
	if err != nil {
		client.Kill()
		RPCLog("Launch: Dispense failed after %s: %v", time.Since(start), err)
		return nil, nil, fmt.Errorf("dispense plugin %s: %w", binPath, err)
	}
	RPCLog("Launch: Dispense OK in %s", time.Since(start))

	p, ok := raw.(Plugin)
	if !ok {
		client.Kill()
		RPCLog("Launch: bad type %T", raw)
		return nil, nil, fmt.Errorf("plugin %s has unexpected type %T", binPath, raw)
	}

	RPCLog("Launch success total=%s", time.Since(start))
	return client, p, nil
}

// Serve runs the plugin RPC server. Blocks forever (until the host kills the process).
func Serve(impl Plugin) {
	_ = OpenRPCLog("redis-rpc")
	RPCLog("Serve: starting plugin process")

	logPath := filepath.Join(pluginapi.LogsDir(), "go-plugin-redis.log")
	_ = os.MkdirAll(pluginapi.LogsDir(), 0755)
	logFile, err := os.OpenFile(logPath, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0644)
	var logger hclog.Logger
	if err == nil {
		logger = hclog.New(&hclog.LoggerOptions{
			Name:   "omo-redis-plugin",
			Output: logFile,
			Level:  hclog.Debug,
		})
	}

	plugin.Serve(&plugin.ServeConfig{
		HandshakeConfig: Handshake,
		Plugins:         ServePluginMap(impl),
		Logger:          logger,
	})
}
