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

## Configuration

The sample configurarion file is `conf/sample.ini`.

Or 

```bash
./logserver -sampleConfig
```

For customize configurations and show it:

```bash
./logserver -conf conf/sample.init -printConfig
```

## CLI Options

```bash
./logserver -h
Usage of ./logserver:
  -config string
        The Config File. Example: cfg.init
  -printConfig
        Print Default INI Config
  -sampleConfig
        Print Sample INI Config
```

