package main

import (
	"omo/pkg/pluginrpc"
	"omo/plugins/git"
)

func main() {
	pluginrpc.Serve(git.NewService())
}
