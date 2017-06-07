package routes

import (
	"gopkg.in/macaron.v1"
	"github.com/go-macaron/sockets"
	"github.com/moisespsena/logserver/core"
	"strings"
	"github.com/gorilla/websocket"
	"time"
	"encoding/json"
	"bytes"
	"fmt"
	"os"
)

func Init(s *core.LogServer, m *macaron.Macaron) {
	log := core.Log

	m.Get(s.Route("/favicon.ico"), func(ctx *macaron.Context) (int, string) {
		return 404, "Not Found"
	})

	m.Get(s.Route("/file/*.*"), func(ctx *macaron.Context) {
		ext := ctx.Params(":ext")
		key := ctx.Params(":path")

		if ext != "" {
			key += "." + ext
		}

		file, err := s.RequestFileProvider(s.Files, ctx, key)
		rootUrl := strings.Replace(s.ServerUrl, "HOST", ctx.Req.Host, 1)

		status := 200

		if err != nil {
			if os.IsNotExist(err) {
				status = 404
				ctx.Data["fileNotExists"] = true
			} else {
				ctx.Data["err"] = fmt.Sprint(err)
			}
		}

		ctx.Data["file"] = file
		ctx.Data["ROOT_URL"] = rootUrl
		ctx.Data["WS_URL"] = strings.Replace(rootUrl, "http", "ws", 1) + "/ws/file/" + key
		ctx.Data["STATIC_URL"] = rootUrl + "/static"
		ctx.HTML(status, "wsui") // 200 is the response code.
	})

	files := s.Files

	m.Get(s.Route("/ws/file/*.*"), sockets.Messages(), func(ctx *macaron.Context, receiver <-chan string, senderChan chan<- string, done <-chan bool, disconnect chan<- int, errorChannel <-chan error) {
		ext := ctx.Params(":ext")
		key := ctx.Params(":path")

		if ext != "" {
			key += "." + ext
		}

		file, err := s.RequestFileProvider(s.Files, ctx, key)
		sender := &core.Sender{senderChan}

		if err != nil {
			sender.Send(core.FLASH, err)
			sender.SendClose()
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

		sender.Send(core.FOLLOW, file.Follow())

		if file.Written != nil {
			b, _ := json.Marshal(file.Written)
			sender.Send(core.FILE_INFO, bytes.NewBuffer(b).String())
		}

		if !file.Follow() {
			file.Cat(sender)
			sender.SendClose()
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

		client := files.AddClient(file, &core.Client{Sender: sender})

		for {
			select {
			case <-receiver:
				client.RenewTimeout()
			case <-client.TimeOut:
				if client.Fc.Follow.IsRunning() {
					client.RenewTimeout()
				} else {
					sender.SendClose()
					client.CloseTimeOut = time.After(5 * time.Second)
				}
			case <-client.CloseTimeOut:
				disconnect <- websocket.CloseNormalClosure
			case <-done:
				if client.Exists() {
					client.Remove()
				}
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
