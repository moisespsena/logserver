package cli

import (
	"github.com/moisespsena/logserver/core"
	"flag"
	"io"
	"path/filepath"
	"os"
)

func Init(s *core.LogServer) (err error) {
	flag.StringVar(&s.ConfigFile, "config", "",
		"The Config File. Example: cfg.init")
	printConfig := flag.Bool("printConfig", false,
		"Print Default INI Config")
	sampleConfig := flag.Bool("sampleConfig", false,
		"Print Sample INI Config")

	flag.Parse()

	if *sampleConfig {
		os.Stdout.WriteString(core.SampleConfig())
		return io.EOF
	}

	if s.ConfigFile != "" {
		s.ConfigFile = filepath.Clean(s.ConfigFile)
		err = s.LoadConfig()
	}

	if err == nil && *printConfig {
		os.Stdout.WriteString(s.ConfigString())
		return io.EOF
	}

	return err
}
