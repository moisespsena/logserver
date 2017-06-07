package util

import (
	"os/exec"
	"io/ioutil"
	"regexp"
	"bufio"
	"io"
	"strings"
	"strconv"
)

type IsWritenResult struct {
	Is   bool
	Cmd  string
	Pid  int
	User string
}

var lsofRegex, _ = regexp.Compile("\\s+")

func IsWritten(path string) (IsWritenResult, error) {
	cmd := exec.Command("lsof", "--", path)
	stdout, err := cmd.StdoutPipe()
	stderr, err := cmd.StderrPipe()

	var rNil IsWritenResult

	if err = cmd.Start(); err != nil {
		return rNil, err
	}

	r := &IsWritenResult{}

	reader := bufio.NewReader(stdout)

	first := true

	for {
		line, err := reader.ReadString('\n')

		if err != nil {
			if err == io.EOF {
				break
			}
			return rNil, err
		}

		if first {
			first = false
			continue
		}

		data := lsofRegex.Split(line, 5)

		if strings.ContainsAny(data[3], "uw") {
			r.Is = true
			r.Cmd = data[0]
			r.User = data[2]
			r.Pid, err = strconv.Atoi(data[1])

			if err != nil {
				return rNil, err
			}
			break
		}
	}

	if err != nil {
		return rNil, err
	}

	if err = cmd.Wait(); err != nil {
		ioutil.ReadAll(stdout)
		ioutil.ReadAll(stderr)
		if _, ok := err.(*exec.ExitError); ok {
			return *r, nil
		}
		return rNil, err
	}

	return *r, nil
}
