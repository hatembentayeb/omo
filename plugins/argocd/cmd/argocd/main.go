package main

import (
	"omo/pkg/pluginrpc"
	"omo/plugins/argocd"
)

func main() {
	pluginrpc.Serve(argocd.NewService())
}
