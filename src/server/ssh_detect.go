package server

import (
	"fmt"
	"os/exec"
	"strings"
)

// RemotePane describes the ssh session running in a tmux pane.
type RemotePane struct {
	User string // may be "" if argv didn't qualify with user@
	Host string
	Port string // "" if default
	Key  string // -i identity file, if any
}

// Addr returns the user@host form suitable for passing to ssh.
func (r RemotePane) Addr() string {
	if r.User == "" {
		return r.Host
	}
	return r.User + "@" + r.Host
}

// detectSSHPane probes tmux pane `target` and returns RemotePane if its
// foreground process is an ssh client. Returns (nil, nil) if not ssh.
//
// The probe follows this flow (see notes/plans/cross-device-mic-relay.md):
//  1. tmux display-message -p -t <target> '#{pane_tty}' → tty path
//  2. ps -t <tty> -o tpgid= → terminal foreground process group id
//  3. ps -t <tty> -o pid=,pgid=,comm=,args= → pick the row whose pgid == tpgid
//  4. parse argv; if comm is "ssh", extract user/host/port/key
func detectSSHPane(target string) (*RemotePane, error) {
	ttyOut, err := exec.Command("tmux", "display-message", "-p", "-t", target, "#{pane_tty}").Output()
	if err != nil {
		return nil, fmt.Errorf("tmux display-message: %w", err)
	}
	tty := strings.TrimSpace(string(ttyOut))
	if tty == "" {
		return nil, fmt.Errorf("empty pane_tty for %q", target)
	}

	tpgidOut, err := exec.Command("ps", "-t", tty, "-o", "tpgid=").Output()
	if err != nil {
		return nil, fmt.Errorf("ps tpgid: %w", err)
	}
	tpgid := firstNonEmptyField(string(tpgidOut))
	if tpgid == "" {
		return nil, fmt.Errorf("no tpgid for tty %s", tty)
	}

	rowsOut, err := exec.Command("ps", "-t", tty, "-o", "pid=,pgid=,comm=,args=").Output()
	if err != nil {
		return nil, fmt.Errorf("ps rows: %w", err)
	}
	return parseSSHRow(string(rowsOut), tpgid), nil
}

// firstNonEmptyField returns the first whitespace-delimited field of the
// first non-empty line. Used to pick a single value out of `ps -o field=`.
func firstNonEmptyField(s string) string {
	for _, line := range strings.Split(s, "\n") {
		f := strings.Fields(line)
		if len(f) > 0 {
			return f[0]
		}
	}
	return ""
}

// parseSSHRow walks `ps -o pid=,pgid=,comm=,args=` output and returns the
// RemotePane for whichever row has pgid == wantPgid AND comm == "ssh".
// Returns nil if the foreground process isn't ssh.
func parseSSHRow(output, wantPgid string) *RemotePane {
	for _, line := range strings.Split(output, "\n") {
		fields := strings.Fields(line)
		if len(fields) < 4 {
			continue
		}
		pgid := fields[1]
		comm := fields[2]
		if pgid != wantPgid || comm != "ssh" {
			continue
		}
		// fields[3:] is the argv of the ssh process. Parse flags we care
		// about (-i, -p, -l, -o) and grab the first non-flag positional as
		// the destination.
		return parseSSHArgv(fields[3:])
	}
	return nil
}

// parseSSHArgv extracts user/host/port/key from an ssh argv (first token is
// "ssh" itself). Understands the common short flags with values: -i, -p, -l.
// Long-form -oName=value and -o Name value are tolerated by swallowing both.
func parseSSHArgv(argv []string) *RemotePane {
	if len(argv) == 0 || argv[0] != "ssh" {
		return nil
	}
	flagsWithValues := map[string]bool{
		"-i": true, "-p": true, "-l": true, "-o": true,
		"-b": true, "-c": true, "-D": true, "-E": true, "-e": true,
		"-F": true, "-I": true, "-J": true, "-L": true, "-m": true,
		"-O": true, "-Q": true, "-R": true, "-S": true, "-W": true, "-w": true,
	}
	p := &RemotePane{}
	i := 1
	for i < len(argv) {
		tok := argv[i]
		if tok == "--" {
			i++
			break
		}
		if strings.HasPrefix(tok, "-") {
			// -iFILE, -pPORT, -lUSER: value glued to flag
			if len(tok) > 2 && flagsWithValues[tok[:2]] {
				assignFlag(p, tok[:2], tok[2:])
				i++
				continue
			}
			if flagsWithValues[tok] && i+1 < len(argv) {
				assignFlag(p, tok, argv[i+1])
				i += 2
				continue
			}
			// standalone flag with no value (e.g. -v, -q) — skip
			i++
			continue
		}
		// first positional is the destination
		u, h := splitUserHost(tok)
		if u != "" {
			p.User = u
		}
		p.Host = h
		return p
	}
	return nil
}

func assignFlag(p *RemotePane, flag, val string) {
	switch flag {
	case "-i":
		p.Key = val
	case "-p":
		p.Port = val
	case "-l":
		p.User = val
	}
}

func splitUserHost(s string) (string, string) {
	if i := strings.LastIndex(s, "@"); i >= 0 {
		return s[:i], s[i+1:]
	}
	return "", s
}
