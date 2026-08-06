package main

import (
	"omo/pkg/pluginrpc"
	"omo/plugins/redis"
)

func main() {
	pluginrpc.Serve(redis.NewService())
}
