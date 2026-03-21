package ui

import (
	"bufio"
	"fmt"
	"io"
	"os/exec"
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
