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

# Nginx Proxy Pass Example

Your `serverUrl` config:

```yaml
...
serverUrl = PROTO://HOST/sub/path
...
```

Nginx conf:


    location /sub/path/ws/ {
        proxy_set_header X-Real-IP       $remote_addr;
        proxy_set_header X-Forwarded-For $proxy_add_x_forwarded_for;
        proxy_set_header Upgrade         websocket;
        proxy_set_header Connection      upgrade;
        proxy_set_header Host            $host:$server_port;
        proxy_set_header Origin          $scheme://$host:$server_port;
        proxy_http_version               1.1;
        proxy_pass                       http://localhost:4000;
    }

    location /sub/path {
        proxy_set_header X-Real-IP       $remote_addr;
        proxy_set_header X-Forwarded-For $proxy_add_x_forwarded_for;
        proxy_set_header                 Host $host:$server_port;
        proxy_set_header Origin          $scheme://$host:$server_port;
        proxy_http_version               1.1;
        proxy_pass                       http://localhost:4000;
    }

