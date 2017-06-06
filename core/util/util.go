package util

import (
	"os/exec"
	"io/ioutil"
	"bytes"
	"strings"
	"strconv"
	"regexp"
)

type IsWritenResult struct {
	Is bool
	Cmd string
	Pid int
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

	dataBytes, err := ioutil.ReadAll(stdout)

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

	r.Is = true

	data := lsofRegex.Split(strings.Split(bytes.NewBuffer(dataBytes).String(), "\n")[1], 4)

	r.Cmd = data[0]
	r.User = data[2]
	r.Pid, err = strconv.Atoi(data[1])

	if err != nil {
		return rNil, err
	}


	return *r, nil
}
