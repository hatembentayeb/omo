package main

import (
	"omo/pkg/pluginrpc"
	"omo/plugins/rabbitmq"
)

func main() {
	pluginrpc.Serve(rabbitmq.NewService())
}
