package web

import (
	"gopkg.in/macaron.v1"
	"github.com/moisespsena/logserver/core"
	"github.com/moisespsena/logserver/core/web/routes"
	"os/signal"
	"os"
	"errors"
	"net"
	"fmt"
	"net/http"
	"path/filepath"
	"github.com/go-macaron/pongo2"
)

func Run(s *core.LogServer) (err error) {
	defer s.Log.Info("done")
	s.RootPath = filepath.Clean(s.RootPath)
	m := macaron.Classic()
	macaron.SetConfig(s.Config)

	if os.Getenv("MACARON_ENV") == "" {
		if s.Dev {
			os.Setenv("MACARON_ENV", macaron.DEV)
		} else {
			os.Setenv("MACARON_ENV", macaron.PROD)
		}
	}

	s.M = m
	m.Use(macaron.Static("static", macaron.StaticOptions{
		Prefix:      s.Route("static"),
		SkipLogging: true,
	}))
	m.Use(pongo2.Pongoer())

	if s.PrepareServer != nil {
		err := (s.PrepareServer)(s)
		if err != nil {
			return err
		}
	}

	routes.Init(s, m)

	stateChan := make(chan error)
	signalChan := make(chan os.Signal, 1)
	signal.Notify(signalChan, os.Interrupt)

	var DONE = errors.New("DONE")

	go func() {
		var err error

		if s.UnixSocket {
			listener, err := net.ListenUnix("unix", &net.UnixAddr{s.ServerAddr, "unix"})

			if err != nil {
				stateChan <- err
				return
			}

			err = os.Chmod(s.ServerAddr, os.FileMode(s.SockPerms))

			if err != nil {
				stateChan <- fmt.Errorf("Failed to set permission of unix socket: ", err)
				return
			}

			err = http.Serve(listener, m)
		} else {
			err = http.ListenAndServe(s.ServerAddr, m)
		}

		if err != nil {
			stateChan <- fmt.Errorf("Failed to set permission of unix socket: ", err)
			return
		}

		stateChan <- DONE
	}()

	s.Log.Info("Running...")

	select {
	case err := <-stateChan:
		if err != DONE {
			s.Log.Error(err)
		}
	case sig := <-signalChan:
		s.Log.Warning("Signal: ", sig)
	}

	return nil
}
