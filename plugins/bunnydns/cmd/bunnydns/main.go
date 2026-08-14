package main

import (
	"omo/pkg/pluginrpc"
	"omo/plugins/bunnydns"
)

func main() {
	pluginrpc.Serve(bunnydns.NewService())
}
