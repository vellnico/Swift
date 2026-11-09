package xray

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"
)

// Process is a running xray instance.
type Process interface {
	Wait() error
	Kill() error
	PID() int
}

// Runner abstracts the xray binary so tests can replace it.
type Runner interface {
	// Test validates a config with `xray run -test`.
	Test(ctx context.Context, bin, configPath string) error
	// Start launches `xray run -c configPath`.
	Start(bin, configPath string, output io.Writer) (Process, error)
}

// ExecRunner runs the real binary. Geo files are looked up next to it.
type ExecRunner struct{}

func assetEnv(bin string) []string {
	return append(os.Environ(), "XRAY_LOCATION_ASSET="+filepath.Dir(bin))
}

func (ExecRunner) Test(ctx context.Context, bin, configPath string) error {
	ctx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()
	cmd := exec.CommandContext(ctx, bin, "run", "-test", "-c", configPath)
	cmd.Env = assetEnv(bin)
	out, err := cmd.CombinedOutput()
	if err != nil {
		return &TestError{Output: lastLines(string(out), 6), Err: err}
	}
	return nil
}

func (ExecRunner) Start(bin, configPath string, output io.Writer) (Process, error) {
	cmd := exec.Command(bin, "run", "-c", configPath)
	cmd.Env = assetEnv(bin)
	cmd.Stdout = output
	cmd.Stderr = output
	setProcAttr(cmd)
	if err := cmd.Start(); err != nil {
		return nil, fmt.Errorf("start xray: %w", err)
	}
	return execProcess{cmd}, nil
}

type execProcess struct{ cmd *exec.Cmd }

func (p execProcess) Wait() error { return p.cmd.Wait() }
func (p execProcess) Kill() error { return p.cmd.Process.Kill() }
func (p execProcess) PID() int    { return p.cmd.Process.Pid }

// TestError carries xray's explanation of why a config was rejected.
type TestError struct {
	Output string
	Err    error
}

func (e *TestError) Error() string {
	if e.Output == "" {
		return fmt.Sprintf("xray config test failed: %v", e.Err)
	}
	return "xray config test failed: " + e.Output
}

func (e *TestError) Unwrap() error { return e.Err }

func lastLines(s string, n int) string {
	lines := strings.Split(strings.TrimSpace(s), "\n")
	if len(lines) > n {
		lines = lines[len(lines)-n:]
	}
	return strings.TrimSpace(strings.Join(lines, "\n"))
}

// lineWriter forwards complete lines of process output to fn.
type lineWriter struct {
	fn  func(string)
	buf bytes.Buffer
}

func (w *lineWriter) Write(p []byte) (int, error) {
	w.buf.Write(p)
	for {
		line, err := w.buf.ReadString('\n')
		if err != nil {
			w.buf.WriteString(line)
			return len(p), nil
		}
		if l := strings.TrimSpace(line); l != "" {
			w.fn(l)
		}
	}
}
