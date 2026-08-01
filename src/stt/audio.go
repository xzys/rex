package stt

import (
	"bytes"
	"io"
	"log/slog"
	"os/exec"
	"sync"
)

// AudioCapture manages an arecord subprocess for PCM capture.
type AudioCapture struct {
	cmd    *exec.Cmd
	buf    bytes.Buffer
	mu     sync.Mutex
	stdout io.ReadCloser
}

func NewAudioCapture() *AudioCapture {
	return &AudioCapture{}
}

// Start initialises the mic and launches arecord. Blocks until arecord is running.
func (a *AudioCapture) Start() error {
	// Enable mic boost (disabled at boot on TrimUI Brick)
	exec.Command("amixer", "-c", "0", "cset", "numid=12", "1").Run()
	exec.Command("amixer", "-c", "0", "cset", "numid=13", "1").Run()

	a.mu.Lock()
	a.buf.Reset()
	a.mu.Unlock()

	a.cmd = exec.Command("arecord",
		"-D", "hw:0,0",
		"-f", "S16_LE",
		"-r", "16000",
		"-c", "1",
		"-t", "raw",
	)

	var err error
	a.stdout, err = a.cmd.StdoutPipe()
	if err != nil {
		return err
	}

	if err := a.cmd.Start(); err != nil {
		return err
	}

	// Read PCM data in background
	go func() {
		tmp := make([]byte, 4096)
		for {
			n, err := a.stdout.Read(tmp)
			if n > 0 {
				a.mu.Lock()
				a.buf.Write(tmp[:n])
				a.mu.Unlock()
			}
			if err != nil {
				return
			}
		}
	}()

	slog.Info("arecord started")
	return nil
}

// Drain returns and clears the PCM captured so far without stopping arecord.
func (a *AudioCapture) Drain() []byte {
	a.mu.Lock()
	defer a.mu.Unlock()
	if a.buf.Len() == 0 {
		return nil
	}
	data := make([]byte, a.buf.Len())
	copy(data, a.buf.Bytes())
	a.buf.Reset()
	return data
}

// Stop kills arecord and returns the captured raw PCM bytes.
func (a *AudioCapture) Stop() []byte {
	if a.cmd == nil || a.cmd.Process == nil {
		return nil
	}

	a.cmd.Process.Kill()
	a.cmd.Wait()
	a.cmd = nil

	a.mu.Lock()
	data := make([]byte, a.buf.Len())
	copy(data, a.buf.Bytes())
	a.buf.Reset()
	a.mu.Unlock()

	slog.Info("arecord stopped", "bytes", len(data))
	return data
}
