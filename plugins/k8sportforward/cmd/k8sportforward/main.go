package main

import (
	"omo/pkg/pluginrpc"
	"omo/plugins/k8sportforward"
)

func main() {
	pluginrpc.Serve(k8sportforward.NewService())
}
