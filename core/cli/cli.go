package cli

import (
	"github.com/moisespsena/logserver/core/types"
	"flag"
	"strings"
	"fmt"
	"log"
	"net/url"
)

func Init(g *types.Global) *types.Global {
	flag.StringVar(&g.ServerAddr, "server-addr", "0.0.0.0:4000", "The server address. Example: 0.0.0.0:80, unix://file.sock")
	flag.StringVar(&g.ServerUrl, "server-url", "http://HOST", "The client server url. Example: http://HOST/server")
	flag.IntVar(&g.SockPerms, "sock-perms", 0666, "The unix sock file perms. Example: 0666")

	flag.Parse()


	if strings.HasPrefix(g.ServerAddr, "unix://") {
		g.UnixSocket = true
		g.ServerAddr = strings.TrimLeft( g.ServerAddr,"unix://")
	}

	fmt.Println("serverAddr:", g.ServerAddr)
	fmt.Println("serverUrl:", g.ServerUrl)
	fmt.Println("sockPerms:", g.SockPerms)

	if g.UnixSocket {
		fmt.Println("useUnixSocket? true")
	} else {
		fmt.Println("useUnixSocket? false")
	}

	u, err := url.Parse(g.ServerUrl)

	if err != nil {
		log.Fatal(err)
	}

	g.Path = u.Path

	return g
}
