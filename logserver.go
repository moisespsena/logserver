package main

import (
	"github.com/moisespsena/logserver/core/cli"
	"github.com/moisespsena/logserver/core"
	"github.com/moisespsena/logserver/core/web"
)

func main() {
	s := core.NewServer()

	if err := cli.Init(s); err != nil {
		panic(err)
	}

	core.InitLog(s.LogLevel)

	web.Run(s)
}
