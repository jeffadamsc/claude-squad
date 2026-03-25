package git

import (
	"fmt"
	"os/exec"
	"strings"
)

// CommandExecutor abstracts command execution for local vs remote.
type CommandExecutor interface {
	Run(dir string, name string, args ...string) ([]byte, error)
}

// LocalExecutor runs commands via os/exec.
type LocalExecutor struct{}

func (e *LocalExecutor) Run(dir string, name string, args ...string) ([]byte, error) {
	cmd := exec.Command(name, args...)
	if dir != "" {
		cmd.Dir = dir
	}
	return cmd.CombinedOutput()
}

// RemoteExecutor runs commands over SSH.
type RemoteExecutor struct {
	RunCmd func(cmd string) (string, error)
}

func (e *RemoteExecutor) Run(dir string, name string, args ...string) ([]byte, error) {
	cmd := buildRemoteCommand(dir, name, args...)
	out, err := e.RunCmd(cmd)
	return []byte(out), err
}

func buildRemoteCommand(dir string, name string, args ...string) string {
	cmd := name
	if len(args) > 0 {
		cmd += " " + strings.Join(args, " ")
	}
	if dir != "" {
		cmd = fmt.Sprintf("cd %s && %s", shellEscape(dir), cmd)
	}
	return cmd
}

func shellEscape(s string) string {
	return "'" + strings.ReplaceAll(s, "'", "'\"'\"'") + "'"
}

// defaultExecutor is used when no executor is specified.
var defaultExecutor CommandExecutor = &LocalExecutor{}
