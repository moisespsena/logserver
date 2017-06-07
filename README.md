# LogServer

The Go server for monitoring log files.

## Install

```bash
go get -U github.com/moisespsena/logserver
cd $GOPATH/src/github.com/moisespsena/logserver
go build
```

## Run

```bash
./logserver
```

Access http://localhost:4000/file/test.log

## CLI Options

```bash
./logserver -h
Usage of ./logserver:
  -logLevel int
        0=CRITICAL, 1=ERROR, 2=WARNING, 3=NOTICE, 4=INFO, 5=DEBUG (default 4)
  -root string
        Root path of log files (default "./root")
  -serverAddr string
        The server address. Example: 0.0.0.0:80, unix://file.sock (default "0.0.0.0:4000")
  -serverUrl string
        The client server url. Example: http://HOST/server (default "http://HOST")
  -sockPerms string
        The unix sock file perms. Example: 0666 (default "0666")
```

