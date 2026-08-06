package main

import (
	"omo/pkg/pluginrpc"
	"omo/plugins/sysprocess"
)

func main() {
	pluginrpc.Serve(sysprocess.NewService())
}
