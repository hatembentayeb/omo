package pluginrpc

import (
	"net/rpc"

	"omo/pkg/pluginapi"
)

// SecretsRPCServer serves pluginapi.SecretsProvider over a MuxBroker stream.
// Registered as "Plugin" by go-plugin AcceptAndServe.
type SecretsRPCServer struct {
	Impl pluginapi.SecretsProvider
}

func (s *SecretsRPCServer) Get(path string, reply **pluginapi.SecretEntry) error {
	entry, err := s.Impl.Get(path)
	if err != nil {
		return err
	}
	*reply = entry
	return nil
}

type putArgs struct {
	Path  string
	Entry *pluginapi.SecretEntry
}

func (s *SecretsRPCServer) Put(args putArgs, _ *interface{}) error {
	return s.Impl.Put(args.Path, args.Entry)
}

func (s *SecretsRPCServer) Delete(path string, _ *interface{}) error {
	return s.Impl.Delete(path)
}

func (s *SecretsRPCServer) List(prefix string, reply *[]string) error {
	paths, err := s.Impl.List(prefix)
	if err != nil {
		return err
	}
	*reply = paths
	return nil
}

func (s *SecretsRPCServer) Reload(_ interface{}, _ *interface{}) error {
	return s.Impl.Reload()
}

func (s *SecretsRPCServer) Close(_ interface{}, _ *interface{}) error {
	// Host owns the secrets provider lifecycle; plugins must not close it.
	return nil
}

// SecretsRPCClient is the plugin-side SecretsProvider over RPC.
type SecretsRPCClient struct {
	client *rpc.Client
}

func (c *SecretsRPCClient) Get(path string) (*pluginapi.SecretEntry, error) {
	var entry *pluginapi.SecretEntry
	err := c.client.Call("Plugin.Get", path, &entry)
	return entry, err
}

func (c *SecretsRPCClient) Put(path string, entry *pluginapi.SecretEntry) error {
	return c.client.Call("Plugin.Put", putArgs{Path: path, Entry: entry}, new(interface{}))
}

func (c *SecretsRPCClient) Delete(path string) error {
	return c.client.Call("Plugin.Delete", path, new(interface{}))
}

func (c *SecretsRPCClient) List(prefix string) ([]string, error) {
	var paths []string
	err := c.client.Call("Plugin.List", prefix, &paths)
	return paths, err
}

func (c *SecretsRPCClient) Reload() error {
	return c.client.Call("Plugin.Reload", new(interface{}), new(interface{}))
}

func (c *SecretsRPCClient) Close() error {
	return c.client.Call("Plugin.Close", new(interface{}), new(interface{}))
}
