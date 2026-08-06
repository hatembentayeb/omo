package main

import (
	"omo/pkg/pluginrpc"
	"omo/plugins/docker"
)

func main() {
	pluginrpc.Serve(docker.NewService())
}
