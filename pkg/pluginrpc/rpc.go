package pluginrpc

import (
	"net/rpc"
	"time"

	"omo/pkg/pluginapi"

	"github.com/hashicorp/go-plugin"
)

// RPCClient talks to a plugin process over net/rpc.
type RPCClient struct {
	client *rpc.Client
}

func (c *RPCClient) GetMetadata() (pluginapi.PluginMetadata, error) {
	RPCLog("RPCClient.GetMetadata →")
	start := time.Now()
	var meta pluginapi.PluginMetadata
	err := c.client.Call("Plugin.GetMetadata", new(interface{}), &meta)
	RPCLog("RPCClient.GetMetadata ← err=%v dur=%s name=%s", err, time.Since(start), meta.Name)
	return meta, err
}

func (c *RPCClient) Configure(req ConfigureRequest) error {
	RPCLog("RPCClient.Configure → keys=%v", mapKeys(req.Settings))
	start := time.Now()
	err := c.client.Call("Plugin.Configure", req, new(interface{}))
	RPCLog("RPCClient.Configure ← err=%v dur=%s", err, time.Since(start))
	return err
}

func (c *RPCClient) GetView(req ViewRequest) (ViewData, error) {
	RPCLog("RPCClient.GetView → view=%q", req.View)
	start := time.Now()
	var view ViewData
	err := c.client.Call("Plugin.GetView", req, &view)
	RPCLog("RPCClient.GetView ← err=%v dur=%s status=%q rows=%d", err, time.Since(start), view.Status, len(view.Rows))
	return view, err
}

func (c *RPCClient) DoAction(req ActionRequest) (ActionResult, error) {
	RPCLog("RPCClient.DoAction → action=%s", req.Action)
	start := time.Now()
	var result ActionResult
	err := c.client.Call("Plugin.DoAction", req, &result)
	RPCLog("RPCClient.DoAction ← err=%v dur=%s ok=%v", err, time.Since(start), result.OK)
	return result, err
}

func (c *RPCClient) Stop() error {
	RPCLog("RPCClient.Stop →")
	err := c.client.Call("Plugin.Stop", new(interface{}), new(interface{}))
	RPCLog("RPCClient.Stop ← err=%v", err)
	return err
}

// RPCServer exposes Plugin over net/rpc inside the plugin process.
type RPCServer struct {
	Impl   Plugin
	broker *plugin.MuxBroker
}

func (s *RPCServer) GetMetadata(_ interface{}, resp *pluginapi.PluginMetadata) error {
	RPCLog("RPCServer.GetMetadata")
	meta, err := s.Impl.GetMetadata()
	if err != nil {
		return err
	}
	*resp = meta
	return nil
}

func (s *RPCServer) Configure(req ConfigureRequest, _ *interface{}) error {
	RPCLog("RPCServer.Configure keys=%v", mapKeys(req.Settings))
	return s.Impl.Configure(req)
}

func (s *RPCServer) GetView(req ViewRequest, resp *ViewData) error {
	RPCLog("RPCServer.GetView view=%q", req.View)
	start := time.Now()
	view, err := s.Impl.GetView(req)
	RPCLog("RPCServer.GetView done err=%v dur=%s status=%q rows=%d", err, time.Since(start), view.Status, len(view.Rows))
	if err != nil {
		return err
	}
	*resp = view
	return nil
}

func (s *RPCServer) DoAction(req ActionRequest, resp *ActionResult) error {
	RPCLog("RPCServer.DoAction action=%s", req.Action)
	result, err := s.Impl.DoAction(req)
	if err != nil {
		return err
	}
	*resp = result
	return nil
}

func (s *RPCServer) Stop(_ interface{}, _ *interface{}) error {
	RPCLog("RPCServer.Stop")
	return s.Impl.Stop()
}

func mapKeys(m map[string]string) []string {
	if m == nil {
		return nil
	}
	keys := make([]string, 0, len(m))
	for k := range m {
		if k == "password" {
			keys = append(keys, "password=***")
			continue
		}
		keys = append(keys, k+"="+m[k])
	}
	return keys
}
