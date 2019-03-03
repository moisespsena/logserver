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
	"gopkg.in/ini.v1"
	"strconv"
	"path/filepath"
	"net/url"
	"strings"
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
	OutKey               string
	ErrKey               string
	Written              *util.IsWritenResult
	FollowRequireWritten bool
}

type LogServer struct {
	SiteName            string
	SiteTitle           string
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
	HomeHandler         interface{}
	Files               *Files
	Log                 *logging.Logger
	Data                map[string]interface{}
	RootPath            string
	ConfigFile          string
	OtherConfigFiles    []interface{}
	Config              *ini.File
	Dev                 bool
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
	fpath, err := filepath.Abs(path.Join(srv.RootPath, filename))

	if err != nil {
		return err
	}

	var (
		files []string
	)

	stat, err := os.Stat(fpath)

	if err != nil {
		if os.IsNotExist(err) {
			_, err2 := os.Stat(fpath + ".out")
			_, err3 := os.Stat(fpath + ".err")

			if err2 == nil {
				file.OutPath = fpath + ".out"
				file.OutKey = filename + ".out"
				files = append(files, file.OutPath)
			}

			if err3 == nil {
				file.ErrPath = fpath + ".err"
				file.ErrKey = filename + ".err"
				files = append(files, file.ErrPath)
			}

			if err2 != nil && err3 != nil {
				return err
			}
		} else {
			return err
		}
	} else if stat.Mode().IsDir() {
		return errors.New("'" + filename + "' is a directory.")
	} else {
		file.OutPath = fpath
		file.OutKey = filename
	}

	files = append(files, fpath)
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
		ServerAddr:          "0.0.0.0:4000",
		ServerUrl:           "PROTO://HOST",
		RootPath:            "./data",
		SiteName:            "Log Server",
		SiteTitle:           "Log Server",
		SockPerms:           0666,
		LogLevel:            logging.INFO,
	}

	s.NewFiles()

	return s
}

func (s *LogServer) NewFiles() *Files {
	s.Files = &Files{s, make(map[string]*FileFollowChan), make(chan bool)}
	return s.Files
}

func (s *LogServer) SetServerUrl(serverUrl string) error {
	u, err := url.Parse(serverUrl)

	if err != nil {
		return err
	}

	s.Path = u.Path
	return nil
}

func ParseConfigValue(dest interface{}, value interface{},
	parse func(value string) (interface{}, error)) (interface{}, error) {
	expected := reflect.TypeOf(dest).Kind()
	vt := reflect.TypeOf(value).Kind()

	if vt != expected {
		switch vt {
		case reflect.String:
			newValue, err := parse(value.(string))
			if err != nil {
				return nil, err
			}
			dest = newValue
			return newValue, nil
		default:
			return nil, fmt.Errorf("unexpected type %T", value)
		}
	}

	dest = value
	return value, nil
}

func (s *LogServer) ParseConfig(cfg map[string]interface{}) (err error) {
	if v, ok := cfg["siteName"]; ok {
		s.SiteName = v.(string)
	}
	if v, ok := cfg["siteTitle"]; ok {
		s.SiteTitle = v.(string)
	}
	if v, ok := cfg["serverAddr"]; ok {
		s.ServerAddr = fmt.Sprint(v)

		if strings.HasPrefix(s.ServerAddr, "unix:") {
			s.UnixSocket = true
			s.ServerAddr = strings.TrimLeft(s.ServerAddr, "unix:")
		}
	}
	if v, ok := cfg["serverUrl"]; ok {
		err = s.SetServerUrl(v.(string))

		if err != nil {
			return err
		}
	}
	if v, ok := cfg["root"]; ok {
		s.RootPath = filepath.Clean(v.(string))
	}
	if v, ok := cfg["sockPerms"]; ok {
		_, err = ParseConfigValue(&s.ServerAddr, v, func(value string) (interface{}, error) {
			i64, err := strconv.ParseInt(value, 8, 0)
			if err != nil {
				return nil, err
			}
			return int(i64), nil
		})

		if err != nil {
			return err
		}
	}
	if v, ok := cfg["logLevel"]; ok {
		var logLevel int64
		var x interface{}

		x, err = ParseConfigValue(&logLevel, v, func(value string) (interface{}, error) {
			i, err := strconv.ParseInt(value, 10, 0)
			if err == nil {
				if i >= 0 && i <= 5 {
					return i, nil
				}
				return nil, errors.New("logLevel value isn't between 0 and 5")
			}
			return nil, err
		})

		if err != nil {
			return err
		}

		s.LogLevel = logging.Level(x.(int64))
	}

	if v, ok := cfg["dev"]; ok {
		ParseConfigValue(&s.Dev, v, func(value string) (interface{}, error) {
			if value == "true" {
				return true, err
			}
			return false, nil
		})
	}

	return err
}

func (s *LogServer) PrintConfig() {
	fmt.Println("-------------------------------------------")
	fmt.Println("siteName:      ", s.SiteName)
	fmt.Println("siteTitle:     ", s.SiteTitle)
	fmt.Println("serverAddr:    ", s.ServerAddr)
	fmt.Println("root:          ", s.RootPath)
	fmt.Println("serverUrl:     ", s.ServerUrl)
	fmt.Println("serverPath:    ", s.Path)
	fmt.Println("logLevel:      ", s.LogLevel)
	fmt.Println("useUnixSocket? ", s.UnixSocket)
	if s.UnixSocket {
		fmt.Printf("sockPerms:      0%o\n", s.SockPerms)
	}
	fmt.Println("-------------------------------------------")
}

func SampleConfig() string {
	return NewServer().ConfigString()
}

func (s *LogServer) ConfigString() string {
	serverUrl := s.ServerUrl

	if s.Path != "" {
		serverUrl += s.Path
	}
	return fmt.Sprintf("[logserver]\n"+
		"siteName = %v\n"+
		"siteTitle = %v\n"+
		"serverAddr = %v\n"+
		"serverUrl = %v\n"+
		"root = %v\n"+
		"sockPerms = 0%o\n"+
		"logLevel = %v\n"+
		"dev = %v\n",
		s.SiteName,
		s.SiteTitle,
		s.ServerAddr,
		serverUrl,
		s.RootPath,
		s.SockPerms,
		int(s.LogLevel),
		s.Dev)
}

func (s *LogServer) LoadConfigFromStringMap(m map[string]string) (err error) {
	arg := make(map[string]interface{})

	for k, v := range m {
		arg[k] = v
	}

	return s.ParseConfig(arg)
}

func (s *LogServer) LoadConfig() (err error) {
	if s.ConfigFile != "" || len(s.OtherConfigFiles) > 0 {
		s.Config, err = ini.Load(s.ConfigFile, s.OtherConfigFiles...)
		if err == nil {
			section, err := s.Config.GetSection("logserver")
			if err == nil {
				err = s.LoadConfigFromStringMap(section.KeysHash())
			}
		}
	} else {
		s.Config = ini.Empty()
	}
	return err
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
