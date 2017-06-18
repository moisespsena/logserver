package main

import (
	"github.com/moisespsena/logserver/core/cli"
	"github.com/moisespsena/logserver/core"
	"github.com/moisespsena/logserver/core/web"
	"io"
)

func mainCallback(callback func(server *core.LogServer)) {
	s := core.NewServer()

	if err := cli.Init(s); err != nil {
		if err == io.EOF {
			return
		}
		panic(err)
	}

	core.InitLog(s.LogLevel)

	if callback != nil {
		callback(s)
	}

	web.Run(s)
}

func main() {
	mainCallback(func(server *core.LogServer) {})
}
