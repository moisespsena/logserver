package main

import (
	"github.com/moisespsena/logserver/core/cli"
	"github.com/moisespsena/logserver/core"
	"github.com/moisespsena/logserver/core/web"
	"io"
)

func main() {
	s := core.NewServer()

	if err := cli.Init(s); err != nil {
		if err == io.EOF {
			return
		}
		panic(err)
	}

	core.InitLog(s.LogLevel)
	web.Run(s)
}
