// Package main 是 igo dev 模式的最小冒烟示例。
//
// 用法:
//
//	cd examples/dev_demo
//	go run ../../cmd/igo dev          # watcher 起在 :18999, 子进程跑这个 main
//	curl http://127.0.0.1:18999/_ai/dev | jq
//	curl http://127.0.0.1:8080/_ai/info | jq
//
// 然后修改本文件里的 helloHandler,SSE 会推送 build:start -> build:ok -> reload:done。
package main

import (
	"os"

	"github.com/leebo/igo"
	"github.com/leebo/igo/core"
)

func main() {
	// Trust APP_ADDR verbatim; ":0" picks a random free port that the
	// watcher will discover via /_internal/announce and surface in /_ai/dev.
	addr := os.Getenv("APP_ADDR")
	if addr == "" {
		addr = ":8080"
	}

	app := igo.Simple()

	app.Get("/", helloHandler)
	app.Get("/ping", pingHandler)

	app.PrintRoutes()
	if err := app.Run(addr); err != nil {
		panic(err)
	}
}

func helloHandler(c *igo.Context) {
	c.Success(igo.H{"msg": "hello from dev_demo", "mode": string(core.DetectMode())})
}

func pingHandler(c *igo.Context) {
	c.Success(igo.H{"pong": true})
}
