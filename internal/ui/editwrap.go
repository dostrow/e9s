package ui

import (
	"bufio"
	"fmt"
	"io"
	"os"
	"os/exec"
)

// editorCmd implements tea.ExecCommand for opening a temp file in $EDITOR.
type editorCmd struct {
	tmpPath string
	stdin   io.Reader
	stdout  io.Writer
	stderr  io.Writer
}

// NewEditorCmd creates a command that opens the given temp file in $EDITOR.
func NewEditorCmd(tmpPath string) *editorCmd {
	return &editorCmd{tmpPath: tmpPath}
}

func (e *editorCmd) SetStdin(r io.Reader)  { e.stdin = r }
func (e *editorCmd) SetStdout(w io.Writer) { e.stdout = w }
func (e *editorCmd) SetStderr(w io.Writer) { e.stderr = w }

func (e *editorCmd) Run() error {
	editor := os.Getenv("EDITOR")
	if editor == "" {
		editor = os.Getenv("VISUAL")
	}
	if editor == "" {
		// Try common editors
		for _, candidate := range []string{"nano", "vi", "vim"} {
			if _, err := exec.LookPath(candidate); err == nil {
				editor = candidate
				break
			}
		}
	}
	if editor == "" {
		fmt.Fprintln(e.stdout, "No editor found. Set $EDITOR environment variable.")
		fmt.Fprint(e.stdout, "Press Enter to continue...")
		bufio.NewReader(e.stdin).ReadBytes('\n')
		return fmt.Errorf("no editor found")
	}

	cmd := exec.Command(editor, e.tmpPath)
	cmd.Stdin = e.stdin
	cmd.Stdout = e.stdout
	cmd.Stderr = e.stderr

	return cmd.Run()
}
