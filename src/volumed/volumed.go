// Package volumed listens to the hardware volume rocker on /dev/input/event0
// (sunxi-keyboard, KEY_VOLUMEUP/KEY_VOLUMEDOWN) and adjusts the ALSA `digital
// volume' control via amixer. Knulli handles these keys natively but the
// Debian chroot has no consumer, so we run a tiny listener inside rex.
package volumed

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"log/slog"
	"os"
	"os/exec"
	"strconv"
)

const (
	devicePath = "/dev/input/event0"

	evKey          = 0x01
	keyVolumeDown  = 114
	keyVolumeUp    = 115

	// "digital volume" on the sun50iw10 codec: 0..63, ~1.16dB per step.
	volNumid = 4
	volMin   = 0
	volMax   = 63
	volStep  = 4
)

// Min and Max bound the digital volume control, exported so UI that renders
// volume shares this package's knowledge of the control instead of its own.
const (
	Min = volMin
	Max = volMax
)

// Get reads the current digital volume (Min..Max).
func Get() (int, error) { return readVol() }

// Set writes the digital volume, clamping to Min..Max.
func Set(v int) error {
	if v < volMin {
		v = volMin
	}
	if v > volMax {
		v = volMax
	}
	return writeVol(v)
}

// inputEvent matches Linux struct input_event on aarch64 (24 bytes):
// __kernel_long_t sec, __kernel_long_t usec, __u16 type, __u16 code, __s32 value.
type inputEvent struct {
	Sec   int64
	Usec  int64
	Type  uint16
	Code  uint16
	Value int32
}

func Run() error {
	f, err := os.Open(devicePath)
	if err != nil {
		if os.IsNotExist(err) {
			slog.Info("volumed: device not present, exiting", "device", devicePath)
			return nil
		}
		return fmt.Errorf("open %s: %w", devicePath, err)
	}
	defer f.Close()

	slog.Info("volumed listening", "device", devicePath)

	var ev inputEvent
	for {
		if err := binary.Read(f, binary.LittleEndian, &ev); err != nil {
			return fmt.Errorf("read event: %w", err)
		}
		if ev.Type != evKey || ev.Value == 0 {
			continue
		}
		switch ev.Code {
		case keyVolumeUp:
			adjust(+volStep)
		case keyVolumeDown:
			adjust(-volStep)
		}
	}
}

func adjust(delta int) {
	cur, err := readVol()
	if err != nil {
		slog.Warn("read volume failed", "err", err)
		return
	}
	next := cur + delta
	if next < volMin {
		next = volMin
	}
	if next > volMax {
		next = volMax
	}
	if next == cur {
		return
	}
	if err := writeVol(next); err != nil {
		slog.Warn("write volume failed", "err", err)
		return
	}
	slog.Info("volume", "value", next, "max", volMax)
}

func readVol() (int, error) {
	out, err := exec.Command("amixer", "-c", "0", "cget", "numid="+strconv.Itoa(volNumid)).Output()
	if err != nil {
		return 0, err
	}
	for _, line := range bytes.Split(out, []byte{'\n'}) {
		i := bytes.Index(line, []byte("  : values="))
		if i < 0 {
			continue
		}
		return strconv.Atoi(string(bytes.TrimSpace(line[i+len("  : values="):])))
	}
	return 0, fmt.Errorf("amixer cget: no values= line")
}

func writeVol(v int) error {
	return exec.Command("amixer", "-c", "0", "cset", "numid="+strconv.Itoa(volNumid), strconv.Itoa(v)).Run()
}
