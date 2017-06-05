package main

import (
	"log"
	"net/http"
	"os"
	"net"
	"github.com/moisespsena/logserver/core/cli"
	"github.com/moisespsena/logserver/core/types"
	"github.com/moisespsena/logserver/core/web"
)


func main() {
	g := cli.Init(&types.Global{})

	log.Println("Server is running...")

	m := web.Init(g)

	var err error

	if g.UnixSocket {
		listener, err := net.ListenUnix("unix", &net.UnixAddr{g.ServerAddr, "unix"})

		if err != nil {
			log.Panic(err)
		}

		err = os.Chmod(g.ServerAddr, os.FileMode(g.SockPerms))

		if err != nil {
			log.Fatal(4, "Failed to set permission of unix socket: %v", err)
		}

		err = http.Serve(listener, m)
	} else {
		err = http.ListenAndServe(g.ServerAddr, m)
	}

	if err != nil {
		log.Fatal(4, "Fail to start server: %v", err)
	}
}
