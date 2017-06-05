package routes

import (
	"gopkg.in/macaron.v1"
	"github.com/go-macaron/sockets"
	"log"
	"github.com/moisespsena/logserver/core/types"
	"strings"
	"github.com/gorilla/websocket"
	"time"
)

func Init(G *types.Global, m *macaron.Macaron) {
	m.Get(G.Route("/favicon.ico"), func(ctx *macaron.Context) (int, string) {
		return 404, "Not Found"
	})

	m.Get(G.Route("/file/:fileName"), func(ctx *macaron.Context) {
		fileName := ctx.Params(":fileName")
		rootUrl := strings.Replace(G.ServerUrl, "HOST", ctx.Req.Host, 1)
		ctx.Data["ROOT_URL"] = rootUrl
		ctx.Data["fileName"] = fileName
		ctx.Data["WS_URL"] = strings.Replace(rootUrl, "http", "ws", 1) + "/ws/file/" + fileName
		ctx.Data["STATIC_URL"] = rootUrl + "/static"
		ctx.HTML(200, "wsui") // 200 is the response code.
	})

	files := types.NewFiles()

	m.Get(G.Route("/ws/file/:fileName"), sockets.Messages(), func(ctx *macaron.Context, receiver <-chan string, sender chan<- string, done <-chan bool, disconnect chan<- int, errorChannel <-chan error) {
		fileName := ctx.Params(":fileName")
		client := files.AddClient(fileName, &types.Client{Sender: sender})

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
				log.Print(err)
				client.Remove()
			}
		}
	})
}