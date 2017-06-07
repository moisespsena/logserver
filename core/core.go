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
	"reflect"
	"github.com/pkg/errors"
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

type Sender struct {
	Chan chan<- string
}

type File struct {
	Key                  string
	Info                 string
	follow               bool
	OutPath              string
	ErrPath              string
	Written              *util.IsWritenResult
	FollowRequireWritten bool
}

type LogServer struct {
	ServerAddr          string
	ServerUrl           string
	SockPerms           int
	UnixSocket          bool
	Path                string
	LogLevel            logging.Level
	M                   *macaron.Macaron
	PrepareServer       func(srv *LogServer) (err error)
	FileProvider        func(srv *LogServer, filename string, file *File) (err error)
	RequestFileProvider func(files *Files, ctx *macaron.Context, filename string) (*File, error)
	Files               *Files
	Log                 *logging.Logger
	Data                map[string]interface{}
	RootPath            string
}

type Message struct {
	Out       string `json:"text"`
	Err       string `json:"text"`
	GlobalErr string `json:"text"`
	Text      string `json:"text"`
}

type Files struct {
	Server *LogServer
	Files  map[string]*FileFollowChan
	Timer  chan bool
}

type FileFollowChan struct {
	Files          *Files
	File           *File
	LastId         int
	Chan           chan string
	Clients        map[int]*Client
	LastTailId     int
	Follow         *Follow
	ClientsMonitor []chan bool
}

type Client struct {
	Sender       *Sender
	Id           int
	Fc           *FileFollowChan
	Closed       bool
	TimeOut      <-chan time.Time
	CloseTimeOut <-chan time.Time
	exists       bool
}

type Follow struct {
	Fc        *FileFollowChan
	Id        int
	Running   int
	KillChans []chan bool
}

func DefaultRequestFileProvider(files *Files, ctx *macaron.Context, filename string) (*File, error) {
	return files.GetFile(filename)
}

func DefaultFileProvider(srv *LogServer, filename string, file *File) (err error) {
	fpath := path.Join(srv.RootPath, filename)

	var (
		files []string
	)

	if _, err = os.Stat(fpath); err != nil {
		if os.IsNotExist(err) {
			_, err2 := os.Stat(fpath + ".out")
			_, err3 := os.Stat(fpath + ".err")

			if err2 == nil {
				file.OutPath = fpath + ".out"
				files = append(files, fpath+".out")
			}

			if err3 == nil {
				file.ErrPath = fpath + ".err"
				files = append(files, fpath+".err")
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

	if _, err = file.CheckFollow(); err != nil {
		return err
	}

	return nil
}

func NewServer() (s *LogServer) {
	s = &LogServer{
		Log:                 Log,
		Data:                make(map[string]interface{}),
		FileProvider:        DefaultFileProvider,
		RequestFileProvider: DefaultRequestFileProvider,
	}

	s.NewFiles()

	return s
}

func (s *LogServer) NewFiles() *Files {
	s.Files = &Files{s, make(map[string]*FileFollowChan), make(chan bool)}
	return s.Files
}

func (g *LogServer) Route(path string) string {
	if (g.Path != "") {
		return g.Path + "/" + path
	}
	return path
}

func (fc *FileFollowChan) Send(t int, line interface{}) {
	for _, client := range fc.Clients {
		client.Send(t, line)
	}
}

func (fc *FileFollowChan) IsWritten() bool {
	if fc.File.OutPath != "" {
		if r, err := util.IsWritten(fc.File.OutPath); err == nil && r.Is {
			return true
		}
	}
	if fc.File.ErrPath != "" {
		if r, err := util.IsWritten(fc.File.ErrPath); err == nil && r.Is {
			return true
		}
	}

	return false
}

func (fc *FileFollowChan) ClientMonitor() chan bool {
	c := make(chan bool)
	fc.ClientsMonitor = append(fc.ClientsMonitor, c)
	return c
}

func (fc *FileFollowChan) Start() {
	fc.Follow.Start()

	go func() {
		if fc.File.FollowRequireWritten {
			for ; len(fc.Clients) > 0 && fc.IsWritten(); {
				time.Sleep(2 * time.Second)
			}

			if !fc.IsWritten() {
				fc.Send(FILE_INFO, "file closed")
			}
		} else {
			for ; len(fc.Clients) > 0; {
				time.Sleep(2 * time.Second)
			}
		}

		fc.Follow.Kill()

		for _, c := range fc.ClientsMonitor {
			c <- true
		}
	}()
}

func (f *Follow) KillMonitor() chan bool {
	c := make(chan bool)
	f.KillChans = append(f.KillChans, c)
	return c
}

func (f *Follow) Start() {
	if f.Fc.File.OutPath != "" {
		go f.StartFile(f.Fc.File.OutPath, OUT, GLOBAL_ERR, f.KillMonitor())
	}

	if f.Fc.File.ErrPath != "" {
		go f.StartFile(f.Fc.File.ErrPath, ERR, GLOBAL_ERR, f.KillMonitor())
	}
}

func (f *Follow) Kill() {
	for _, c := range f.KillChans {
		c <- true
	}
}

func (f *Follow) IsRunning() bool {
	return f.Running > 0
}

func (f *Follow) StartFile(fileName string, t int, gt int, kill chan bool) {
	f.Running++

	defer func() {
		f.Running--
	}()

	fc := f.Fc
	fc.LastTailId++

	if _, err := os.Stat(fileName); err != nil {
		if os.IsNotExist(err) {
			line := fmt.Sprintf("File '%v' does not exists.", fileName)
			fc.Send(gt, &line)
			return
		}
	}

	cmd := exec.Command("tail", "-f", "-n", "0", fileName)
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
					fc.Send(gt, &line)
				}
				return
			}
			if line != "" {
				fc.Send(t, &line)
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
					fc.Send(gt, &line)
				}
				return
			}
			if line != "" {
				fc.Send(t, &line)
			}
		}
	}()

	if err != nil {
		line := fmt.Sprint(err)
		fc.Send(gt, &line)
	} else {
		err := cmd.Start()

		if err != nil {
			line := fmt.Sprint(err)
			fc.Send(gt, &line)
			return
		}

		done := make(chan error)

		go func() {
			done <- cmd.Wait()
		}()

		for {
			select {
			case err = <-done:
				return
			case <-kill:
				cmd.Process.Kill()
			}
		}
	}
}

func (f *Files) AddClient(file *File, client *Client) *Client {
	if !file.follow {
		panic(errors.New("file isn't follow."))
	}

	fc, ok := f.Files[file.Key]

	if ok {
		fc.LastId++
	} else {
		fc = &FileFollowChan{
			f,
			file,
			0,
			make(chan string, 200),
			make(map[int]*Client),
			0,
			nil,
			nil,
		}
		fc.Follow = &Follow{
			fc,
			fc.LastTailId,
			0,
			nil,
		}
		fc.LastTailId++
		f.Files[file.Key] = fc
	}

	client.Id = fc.LastId
	client.Fc = fc
	client.RenewTimeout()
	fc.Clients[client.Id] = client
	client.exists = true

	if len(fc.Clients) == 1 {
		fc.Start()
	}

	return client
}

func (client *Client) Exists() bool {
	return client.exists
}

func (client *Client) Remove() {
	if !client.exists {
		panic(errors.New("client does not exists."))
	}

	delete(client.Fc.Clients, client.Id)
	client.exists = false

	if len(client.Fc.Clients) < 1 {
		close(client.Fc.Chan)
		delete(client.Fc.Files.Files, client.Fc.File.Key)
	}
}

const (
	OUT          = 0
	ERR          = 1
	GLOBAL_ERR   = 2
	CLIENT_CLOSE = 3
	FLASH        = 4
	FILE_INFO    = 5
	FOLLOW       = 6
)

func (file *File) Follow() bool {
	return file.follow
}

func (file *File) CheckFollow() (follow bool, err error) {
	written := false

	for _, fpath := range []string{file.OutPath, file.ErrPath} {
		if fpath == "" {
			continue
		}

		isw, err := util.IsWritten(fpath)

		if err != nil {
			return false, err
		}

		if isw.Is {
			written = true
			file.Written = &isw
			break
		}
	}

	file.follow = written || !file.FollowRequireWritten

	return file.follow, nil
}
func (file *File) Cat(sender *Sender) {
	reader := func(path string, t int) {
		inFile, _ := os.Open(path)
		defer inFile.Close()
		scanner := bufio.NewScanner(inFile)
		scanner.Split(bufio.ScanLines)
		var line string

		for scanner.Scan() {
			line = scanner.Text()
			sender.Send(t, &line)
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

func (client *Client) Send(t int, data interface{}) {
	client.Sender.Send(t, data)
}

func (files *Files) Start() {
	go func() {
		time.Sleep(3 * time.Second)
		files.Timer <- true
	}()
}

func (files *Files) GetFile(key string) (*File, error) {
	file := &File{FollowRequireWritten: true, Key: key}
	err := files.Server.FileProvider(files.Server, key, file)
	return file, err
}

func (s *Sender) SendClose() {
	s.Send(CLIENT_CLOSE, "")
}

func (s *Sender) Send(t int, data interface{}) {
	rvalue := reflect.ValueOf(data)

	if rvalue.Kind() == reflect.Ptr {
		data = reflect.Indirect(rvalue)
	}

	s.Chan <- fmt.Sprintf("%02x%v", t, data)
}
