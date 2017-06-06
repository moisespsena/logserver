package routes

import (
	"gopkg.in/macaron.v1"
	"github.com/go-macaron/sockets"
	"github.com/moisespsena/logserver/core"
	"strings"
	"github.com/gorilla/websocket"
	"time"
	"fmt"
)

func Init(s *core.LogServer, m *macaron.Macaron) {
	log := core.Log

	m.Get(s.Route("/favicon.ico"), func(ctx *macaron.Context) (int, string) {
		return 404, "Not Found"
	})

	m.Get(s.Route("/file/:fileName"), func(ctx *macaron.Context) {
		fileName := ctx.Params(":fileName")
		rootUrl := strings.Replace(s.ServerUrl, "HOST", ctx.Req.Host, 1)
		ctx.Data["ROOT_URL"] = rootUrl
		ctx.Data["fileName"] = fileName
		ctx.Data["WS_URL"] = strings.Replace(rootUrl, "http", "ws", 1) + "/ws/file/" + fileName
		ctx.Data["STATIC_URL"] = rootUrl + "/static"
		ctx.HTML(200, "wsui") // 200 is the response code.
	})

	files := s.Files

	m.Get(s.Route("/ws/file/:fileName"), sockets.Messages(), func(ctx *macaron.Context, receiver <-chan string, sender chan<- string, done <-chan bool, disconnect chan<- int, errorChannel <-chan error) {
		fileName := ctx.Params(":fileName")
		file := &core.File{}
		err := s.FileProvider(s, ctx, fileName, file)

		if err != nil {
			sender <- fmt.Sprintf("4%v", err)
			sender <- core.CLIENT_CLOSE
			timer := time.NewTimer(time.Second * 5)
			select {
			case <-done:
				timer.Stop()
				return
			case <-timer.C:
				disconnect <- websocket.CloseNormalClosure
			}
			return
		}

		if !file.Tail {
			file.Cat(sender)
			sender <- core.CLIENT_CLOSE
			timer := time.NewTimer(time.Second * 5)
			select {
			case <-done:
				timer.Stop()
				return
			case <-timer.C:
				disconnect <- websocket.CloseNormalClosure
			}
			return
		}

		client := files.AddClient(fileName, &core.Client{Sender: sender, File:*file})

		for {
			select {
			case <-receiver:
				client.RenewTimeout()
			case <-client.TimeOut:
				if client.Fc.Tail.IsRunning() {
					client.RenewTimeout()
				} else {
					sender <- "3"
					client.CloseTimeOut = time.After(5 * time.Second)
				}
			case <- client.CloseTimeOut:
				disconnect <- websocket.CloseNormalClosure
			case <-done:
				client.Remove()
				// the client disconnected, so you should return / break if the done channel gets sent a message
				return
			case err := <-errorChannel:
				//
				// Uh oh, we received an error. This will happen before a close if the client did not disconnect regularly.
				// Maybe useful if you want to store statistics
				log.Error(err)
				client.Remove()
			}
		}
	})
}