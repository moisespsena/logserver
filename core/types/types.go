package types

import (
	"log"
	"time"
	"fmt"
	"os/exec"
	"bufio"
	"io"
	"os"
)

type Global struct {
	ServerAddr string
	ServerUrl string
	SockPerms int
	UnixSocket bool
	Path string
}

type Message struct {
	Out string `json:"text"`
	Err string `json:"text"`
	GlobalErr string `json:"text"`
	Text string `json:"text"`
}

type Files struct {
	Files map[string]*FileChan
	Timer chan bool
}

type FileChan struct {
	Files    *Files
	FileName string
	LastId   int
	Chan     chan string
	clients  map[int]*Client
	LastTailId int
	Tail *Tail
}

type Client struct {
	Sender chan<- string
	Id     int
	Fc     *FileChan
	Closed bool
	TimeOut <-chan time.Time
	CloseTimeOut <-chan time.Time
}

type Tail struct {
	Fc *FileChan
	Id int
	OutFile string
	ErrFile string
	Running int
}

func (g *Global) Route(path string) string {
	if (g.Path != "") {
		return g.Path + "/" + path
	}
	return path
}

func (fc *FileChan) SendOut(line string)  {
	for _, client := range fc.clients {
		client.Sender <- "0" + line
	}
}

func (fc *FileChan) SendErr(line string)  {
	for _, client := range fc.clients {
		client.Sender <- "1" + line
	}
}
func (fc *FileChan) SendGlobalErr(line string)  {
	for _, client := range fc.clients {
		client.Sender <- "2" + line
	}
}


func (tail *Tail) Start() {
	go tail.StartFile(tail.OutFile, tail.Fc.SendOut, tail.Fc.SendErr)
	go tail.StartFile(tail.ErrFile, tail.Fc.SendErr, tail.Fc.SendErr)
}

func (tail *Tail) IsRunning() bool {
	return tail.Running > 0
}

func (tail *Tail) StartFile(fileName string, send func (data string), sendErr func (data string)) {
	tail.Running++

	defer func() {
		tail.Running--
	}()

	fc := tail.Fc
	fc.LastTailId++

	if _, err := os.Stat(fileName); err != nil {
		if os.IsNotExist(err) {
			fc.SendGlobalErr(fmt.Sprintf("File '%v' does not exists.", fileName))
			return
		}
	}


	cmd := exec.Command("tail", "-f", "-n", "1", fileName)
	stderr, err := cmd.StderrPipe()

	if err != nil {
		log.Fatal(err)
	}

	stdout, err := cmd.StdoutPipe()

	if err != nil {
		log.Fatal(err)
	}

	go func() {
		reader := bufio.NewReader(stdout)
		for {
			line, err := reader.ReadString('\n')

			if err != nil {
				if err != io.EOF {
					fc.SendGlobalErr(fmt.Sprint(err))
				}
				return
			}
			if line != "" {
				send(line)
			}
		}

	}()

	go func() {
		reader := bufio.NewReader(stderr)
		for {
			line, err := reader.ReadString('\n')
			if err != nil {
				if err != io.EOF {
					fc.SendGlobalErr(fmt.Sprint(err))
				}
				return
			}
			if line != "" {
				sendErr(line)
			}
		}
	}()

	if err != nil {
		fc.SendGlobalErr(fmt.Sprint(err))
	} else {
		err := cmd.Start()

		if err != nil {
			fc.SendGlobalErr(fmt.Sprint(err))
			return
		}

		defer cmd.Process.Kill()

		for {
			if len(fc.clients) == 0 {
				break
			}
			time.Sleep(time.Second * 2)
		}

	}
}

func (f *Files) AddClient(fileName string, client *Client) *Client {
	fc, ok := f.Files[fileName]

	if ok {
		fc.LastId++
	} else {
		fc = &FileChan{f, fileName, 0, make(chan string, 200), make(map[int]*Client), 0, nil}
		fc.Tail = &Tail{fc, fc.LastTailId, fileName + ".out", fileName + ".err", 0}
		fc.LastTailId++
		f.Files[fileName] = fc
	}

	client.Id = fc.LastId
	client.Fc = fc
	client.RenewTimeout()
	fc.clients[client.Id] = client

	if len(fc.clients) == 1 {
		fc.Tail.Start()
	}

	return client
}

func (client *Client) Remove() {
	delete(client.Fc.clients, client.Id)

	if len(client.Fc.clients) < 1 {
		close(client.Fc.Chan)
		delete(client.Fc.Files.Files, client.Fc.FileName)
	}
}

func (client *Client) RenewTimeout() {
	client.TimeOut = time.After(5 * time.Second)
}

func NewFiles() *Files {
	return &Files{make(map[string]*FileChan), make(chan bool)}
}

func (files *Files) Start()  {
	go func() {
		time.Sleep(3 * time.Second)
		files.Timer <- true
	}()
}