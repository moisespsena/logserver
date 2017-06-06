package core

import (
	"time"
	"os/exec"
	"bufio"
	"io"
	"github.com/op/go-logging"
	"gopkg.in/macaron.v1"
	"log"
	"os"
	"fmt"
	"path"
	"github.com/moisespsena/logserver/core/util"
)

const LOG_NAME = "[LogServer] "

var Log = logging.MustGetLogger(LOG_NAME)

// Example format string. Everything except the message has a custom color
// which is dependent on the log level. Many fields have a custom output
// formatting too, eg. the time returns the hour down to the milli second.
var format = logging.MustStringFormatter(
	`%{color}%{time:15:04:05.000} %{shortfunc} ▶ %{level:.4s} %{id:04x}%{color:reset} %{message}`,
)

type Password string

func (p Password) Redacted() interface{} {
	return logging.Redact(string(p))
}

func InitLog(level logging.Level) *logging.Logger {
	logging.SetBackend(logging.NewLogBackend(os.Stdout, LOG_NAME, log.LstdFlags))
	logging.SetLevel(level, "")
	logging.SetFormatter(logging.Formatter(format))
	return Log
}


type File struct {
	Name string
	Info string
	Tail bool
	OutPath string
	ErrPath string
}

type LogServer struct {
	ServerAddr    string
	ServerUrl     string
	SockPerms     int
	UnixSocket    bool
	Path          string
	LogLevel      logging.Level
	M             *macaron.Macaron
	PrepareServer func(srv *LogServer) (err error)
	FileProvider  func(srv *LogServer, ctx *macaron.Context, filename string, file *File) error
	Files         *Files
	Log           *logging.Logger
	Data          map[string]interface{}
}

type Message struct {
	Out       string `json:"text"`
	Err       string `json:"text"`
	GlobalErr string `json:"text"`
	Text      string `json:"text"`
}

type Files struct {
	Files map[string]*FileChan
	Timer chan bool
}

type FileChan struct {
	Files      *Files
	FileName   string
	LastId     int
	Chan       chan string
	CatClients    map[int]*Client
	TailClients    map[int]*Client
	LastTailId int
	Tail       *Tail
}

type Client struct {
	Sender       chan<- string
	Id           int
	Fc           *FileChan
	Closed       bool
	TimeOut      <-chan time.Time
	CloseTimeOut <-chan time.Time
	File File
}

type Tail struct {
	Fc      *FileChan
	Id      int
	OutFile string
	ErrFile string
	Running int
}

func DefaultFileProvider(srv *LogServer, ctx *macaron.Context, filename string, file *File) error {
	fpath := path.Join(filename)

	var err error
	var err2 error
	var err3 error

	var files []string

	if _, err = os.Stat(fpath); err != nil {
		if os.IsNotExist(err) {
			if _, err2 := os.Stat(fpath + ".out"); err2 == nil {
				file.OutPath = fpath + ".out"
				files = append(files, fpath + ".out")
			}
			if _, err3 := os.Stat(fpath + ".err"); err3 == nil {
				file.ErrPath = fpath + ".err"
				files = append(files, fpath + ".err")
			}

			if err2 != nil && err3 != nil {
				return err
			}
		} else {
			return err
		}
	} else {
		files = append(files, fpath)
		file.OutPath = fpath
	}

	for _, fpath = range []string{file.OutPath, file.ErrPath} {
		if fpath == "" {
			continue
		}

		isw, err := util.IsWritten(fpath)

		if err != nil {
			return err
		}

		if isw.Is {
			file.Tail = true
			break
		}
	}

	return nil
}

func NewServer() (s *LogServer) {
	s = &LogServer{
		Files:        NewFiles(),
		Log:          Log,
		Data:         make(map[string]interface{}),
		FileProvider: DefaultFileProvider,
	}

	return s
}

func (g *LogServer) Route(path string) string {
	if (g.Path != "") {
		return g.Path + "/" + path
	}
	return path
}

func (fc *FileChan) SendOut(line *string) {
	for _, client := range fc.TailClients {
		client.SendOut(line)
	}
}

func (fc *FileChan) SendErr(line *string) {
	for _, client := range fc.TailClients {
		client.SendErr(line)
	}
}
func (fc *FileChan) SendGlobalErr(line *string) {
	for _, client := range fc.TailClients {
		client.SendGlobalErr(line)
	}
}

func (tail *Tail) Start() {
	go tail.StartFile(tail.OutFile, tail.Fc.SendOut, tail.Fc.SendErr)
	go tail.StartFile(tail.ErrFile, tail.Fc.SendErr, tail.Fc.SendErr)
}

func (tail *Tail) IsRunning() bool {
	return tail.Running > 0
}

func (tail *Tail) StartFile(fileName string, send func(data *string), sendErr func(data *string)) {
	tail.Running++

	defer func() {
		tail.Running--
	}()

	fc := tail.Fc
	fc.LastTailId++

	if _, err := os.Stat(fileName); err != nil {
		if os.IsNotExist(err) {
			line := fmt.Sprintf("File '%v' does not exists.", fileName)
			fc.SendGlobalErr(&line)
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
					line := fmt.Sprint(err)
					fc.SendGlobalErr(&line)
				}
				return
			}
			if line != "" {
				send(&line)
			}
		}

	}()

	go func() {
		reader := bufio.NewReader(stderr)
		for {
			line, err := reader.ReadString('\n')
			if err != nil {
				if err != io.EOF {
					line = fmt.Sprint(err)
					fc.SendGlobalErr(&line)
				}
				return
			}
			if line != "" {
				sendErr(&line)
			}
		}
	}()

	if err != nil {
		line := fmt.Sprint(err)
		fc.SendGlobalErr(&line)
	} else {
		err := cmd.Start()

		if err != nil {
			line := fmt.Sprint(err)
			fc.SendGlobalErr(&line)
			return
		}

		defer cmd.Process.Kill()

		for {
			if len(fc.TailClients) == 0 {
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
		fc = &FileChan{
			f,
			fileName,
			0,
			make(chan string, 200),
			make(map[int]*Client),
			make(map[int]*Client),
			0,
			nil,
		}
		fc.Tail = &Tail{
			fc,
			fc.LastTailId,
			client.File.OutPath,
			client.File.ErrPath,
			0,
		}
		fc.LastTailId++
		f.Files[fileName] = fc
	}

	client.Id = fc.LastId
	client.Fc = fc
	client.RenewTimeout()

	if client.File.Tail {
		fc.TailClients[client.Id] = client
		if len(fc.TailClients) == 1 {
			fc.Tail.Start()
		}
	}

	return client
}

func (client *Client) Remove() {
	if _, ok := client.Fc.TailClients[client.Id]; ok {
		delete(client.Fc.TailClients, client.Id)
	} else {
		delete(client.Fc.CatClients, client.Id)
	}

	if len(client.Fc.TailClients) < 1 && len(client.Fc.CatClients) < 1{
		close(client.Fc.Chan)
		delete(client.Fc.Files.Files, client.Fc.FileName)
	}
}

const (
	OUT = "0"
	ERR = "1"
	GLOBAL_ERR = "2"
	CLIENT_CLOSE = "3"
)

func (file *File) Cat(sender chan<-string) {
	reader := func (path string, t string) {
		inFile, _ := os.Open(path)
		defer inFile.Close()
		scanner := bufio.NewScanner(inFile)
		scanner.Split(bufio.ScanLines)

		for scanner.Scan() {
			line := scanner.Text()
			sender <- t + line
		}
	}

	if file.ErrPath != "" {
		reader(file.ErrPath, ERR)
	}

	if file.OutPath != "" {
		reader(file.OutPath, OUT)
	}
}


func (client *Client) RenewTimeout() {
	client.TimeOut = time.After(5 * time.Second)
}

func (client *Client) Send(t string, line *string) {
	client.Sender <- t + *line
}

func (client *Client) SendOut(line *string) {
	client.Send(OUT, line)
}

func (client *Client) SendErr(line *string) {
	client.Send(ERR, line)
}
func (client *Client) SendGlobalErr(line *string) {
	client.Send(GLOBAL_ERR, line)
}


func NewFiles() *Files {
	return &Files{make(map[string]*FileChan), make(chan bool)}
}

func (files *Files) Start() {
	go func() {
		time.Sleep(3 * time.Second)
		files.Timer <- true
	}()
}
