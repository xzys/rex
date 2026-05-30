package utils

import (
	"context"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"syscall"
	"time"

	"github.com/creack/pty"
)

func ResolveRexBinary(ctx context.Context) (string, func() error, error) {
	if envPath := os.Getenv("REX_BIN"); envPath != "" {
		if isExecutable(envPath) {
			return envPath, func() error { return nil }, nil
		}
		return "", nil, fmt.Errorf("REX_BIN is not executable: %s", envPath)
	}

	moduleRoot, err := findModuleRoot(".")
	if err != nil {
		return "", nil, err
	}

	defaultPath := filepath.Join(filepath.Dir(moduleRoot), "rex-local")
	if isExecutable(defaultPath) {
		return defaultPath, func() error { return nil }, nil
	}

	if err := buildRexBinary(ctx); err != nil {
		return "", nil, err
	}

	if isExecutable(defaultPath) {
		return defaultPath, func() error { return nil }, nil
	}

	return "", nil, fmt.Errorf("rex-local not found after build at %s", defaultPath)
}

func StartRexProcess(ctx context.Context, rexPath string, args []string, env []string, stdin io.Reader, stdout, stderr io.Writer) (*exec.Cmd, error) {
	cmd := exec.CommandContext(ctx, rexPath, args...)
	cmd.Stdin = stdin
	cmd.Stdout = stdout
	cmd.Stderr = stderr
	if env != nil {
		cmd.Env = env
	}
	if err := cmd.Start(); err != nil {
		return nil, err
	}
	return cmd, nil
}

func TerminateProcess(cmd *exec.Cmd, timeout time.Duration) error {
	if cmd == nil || cmd.Process == nil {
		return nil
	}

	if err := cmd.Process.Signal(syscall.SIGTERM); err != nil {
		return err
	}

	done := make(chan error, 1)
	go func() { done <- cmd.Wait() }()

	select {
	case err := <-done:
		return err
	case <-time.After(timeout):
		if err := cmd.Process.Kill(); err != nil {
			return err
		}
		return <-done
	}
}

func CreateTempFIFO() (string, func() error, error) {
	dir, err := os.MkdirTemp("", "rex-stdin-")
	if err != nil {
		return "", nil, fmt.Errorf("failed to create temp dir: %w", err)
	}

	path := filepath.Join(dir, "rex-stdin")
	if err := syscall.Mkfifo(path, 0600); err != nil {
		os.RemoveAll(dir)
		return "", nil, fmt.Errorf("failed to create fifo: %w", err)
	}

	return path, func() error { return os.RemoveAll(dir) }, nil
}

func WaitForSocket(path string, timeout time.Duration) error {
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		if _, err := os.Stat(path); err == nil {
			return nil
		}
		time.Sleep(50 * time.Millisecond)
	}
	return fmt.Errorf("timed out waiting for socket: %s", path)
}

func findModuleRoot(startDir string) (string, error) {
	dir, err := filepath.Abs(startDir)
	if err != nil {
		return "", fmt.Errorf("failed to resolve path: %w", err)
	}

	for {
		if _, err := os.Stat(filepath.Join(dir, "go.mod")); err == nil {
			return dir, nil
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			return "", fmt.Errorf("go.mod not found from %s", startDir)
		}
		dir = parent
	}
}

func buildRexBinary(ctx context.Context) error {
	_, filename, _, ok := runtime.Caller(0)
	if !ok {
		return fmt.Errorf("failed to get current file path")
	}

	// currentFileDir is <repo>/src/utils; the Makefile lives at <repo>.
	currentFileDir := filepath.Dir(filename)
	projectRoot := filepath.Dir(filepath.Dir(currentFileDir))

	cmd := exec.CommandContext(ctx, "make", "build-local")
	cmd.Dir = projectRoot
	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("make build-local failed: %w\n%s", err, string(output))
	}
	return nil
}

func isExecutable(path string) bool {
	info, err := os.Stat(path)
	if err != nil {
		return false
	}
	return info.Mode().IsRegular() && info.Mode().Perm()&0111 != 0
}

func StartRexProcessWithPTY(ctx context.Context, rexPath string, args []string, env []string) (*exec.Cmd, *os.File, error) {
	cmd := exec.CommandContext(ctx, rexPath, args...)
	if env != nil {
		cmd.Env = env
	}
	ptmx, err := pty.Start(cmd)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to start process with PTY: %w", err)
	}
	return cmd, ptmx, nil
}
