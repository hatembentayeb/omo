package main

import (
	"omo/pkg/pluginrpc"
	"omo/plugins/k8suser"
)

func main() {
	pluginrpc.Serve(k8suser.NewService())
}
