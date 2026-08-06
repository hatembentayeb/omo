package main

import (
	"omo/pkg/pluginrpc"
	"omo/plugins/kafka"
)

func main() {
	pluginrpc.Serve(kafka.NewService())
}
