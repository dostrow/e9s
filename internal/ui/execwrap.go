package ui

import (
	"bufio"
	"fmt"
	"io"
	"os"
	"os/exec"
	"sort"
	"strings"
)

// execWrapCmd implements tea.ExecCommand. It runs the subprocess with full
// terminal access, then prints a separator and waits for Enter before
// returning control to bubbletea. This keeps the output visible on the
// main screen (outside alt screen) so the user can scroll and copy.
type execWrapCmd struct {
	cmd    *exec.Cmd
	stdin  io.Reader
	stdout io.Writer
	stderr io.Writer
}

func NewExecWrap(pluginPath string, args []string) *execWrapCmd {
	return &execWrapCmd{
		cmd: exec.Command(pluginPath, args...),
	}
}

func NewExecWrapInDirectory(executable string, args []string, directory string, environment map[string]string) *execWrapCmd {
	command := exec.Command(executable, args...)
	command.Dir = directory
	if len(environment) > 0 {
		command.Env = mergeEnvironment(os.Environ(), environment)
	}
	return &execWrapCmd{cmd: command}
}

func mergeEnvironment(base []string, overrides map[string]string) []string {
	merged := make(map[string]string, len(base)+len(overrides))
	for _, value := range base {
		name, current, found := strings.Cut(value, "=")
		if found {
			merged[name] = current
		}
	}
	for name, value := range overrides {
		merged[name] = value
	}
	names := make([]string, 0, len(merged))
	for name := range merged {
		names = append(names, name)
	}
	sort.Strings(names)
	values := make([]string, 0, len(names))
	for _, name := range names {
		values = append(values, name+"="+merged[name])
	}
	return values
}

func (e *execWrapCmd) SetStdin(r io.Reader)  { e.stdin = r }
func (e *execWrapCmd) SetStdout(w io.Writer) { e.stdout = w }
func (e *execWrapCmd) SetStderr(w io.Writer) { e.stderr = w }

func (e *execWrapCmd) Run() error {
	e.cmd.Stdin = e.stdin
	e.cmd.Stdout = e.stdout
	e.cmd.Stderr = e.stderr

	err := e.cmd.Run()

	// Print separator and wait for the user to press Enter
	fmt.Fprintln(e.stdout)
	if err != nil {
		fmt.Fprintf(e.stdout, "Session exited with error: %v\n", err)
	} else {
		fmt.Fprintln(e.stdout, "Session ended.")
	}
	fmt.Fprint(e.stdout, "Press Enter to return to e9s...")

	// Read one line from stdin to wait
	reader := bufio.NewReader(e.stdin)
	reader.ReadBytes('\n')

	return err
}
