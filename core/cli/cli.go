package cli

import (
	"github.com/moisespsena/logserver/core"
	"flag"
	"strings"
	"log"
	"net/url"
	"github.com/op/go-logging"
	"fmt"
	"strconv"
)

func Init(s *core.LogServer) (err error) {
	flag.StringVar(&s.ServerAddr, "serverAddr", "0.0.0.0:4000",
		"The server address. Example: 0.0.0.0:80, unix://file.sock")
	flag.StringVar(&s.ServerUrl, "serverUrl", "http://HOST",
		"The client server url. Example: http://HOST/server")

	var sockPerms string
	var logLevel int

	flag.StringVar(&sockPerms, "sockPerms", "0666",
		"The unix sock file perms. Example: 0666")
	flag.IntVar(&logLevel, "logLevel", int(logging.INFO),
		"0=CRITICAL, 1=ERROR, 2=WARNING, 3=NOTICE, 4=INFO, 5=DEBUG")

	flag.Parse()

	i64, err := strconv.ParseInt(sockPerms, 8, 0)

	if err != nil {
		panic(err)
	}

	s.SockPerms = int(i64)

	s.LogLevel = logging.Level(logLevel)


	if strings.HasPrefix(s.ServerAddr, "unix://") {
		s.UnixSocket = true
		s.ServerAddr = strings.TrimLeft( s.ServerAddr,"unix://")
	}

	fmt.Println("-------------------------------------------")
	fmt.Println("serverAddr:    ", s.ServerAddr)
	fmt.Println("serverUrl:     ", s.ServerUrl)
	fmt.Println("sockPerms:     ", sockPerms)
	fmt.Println("logLevel:      ", s.LogLevel)
	fmt.Println("useUnixSocket? ", s.UnixSocket)
	fmt.Println("-------------------------------------------")

	u, err := url.Parse(s.ServerUrl)

	if err != nil {
		log.Fatal(err)
	}

	s.Path = u.Path

	return nil
}
