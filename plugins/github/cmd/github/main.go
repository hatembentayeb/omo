package main

import (
	"omo/pkg/pluginrpc"
	"omo/plugins/github"
)

func main() {
	pluginrpc.Serve(github.NewService())
}
