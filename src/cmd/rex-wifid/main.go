// rex-wifid is a small host-side daemon that wraps ConnMan D-Bus calls and
// exposes them over a Unix socket (/tmp/rex-wifi.sock). It runs as root on
// the Knulli host so that rex (running inside the Debian chroot as uid 1000)
// can control WiFi without needing D-Bus policy files or cross-chroot binary
// execution.
//
// Protocol: line-delimited JSON, one request per line, one response per
// request. See notes/design/rex-wifid.md for the full spec.
package main

import (
	"bufio"
	"encoding/json"
	"fmt"
	"log"
	"net"
	"os"

	"github.com/godbus/dbus/v5"
	"github.com/xrfr/t-rex/v2/wifi"
)

const sockPath = "/tmp/rex-wifi.sock"

type request struct {
	ID         string `json:"id"`
	Cmd        string `json:"cmd"`
	On         *bool  `json:"on,omitempty"`
	Path       string `json:"path,omitempty"`
	Passphrase string `json:"passphrase,omitempty"`
	SSID       string `json:"ssid,omitempty"`
}

type response struct {
	ID       string        `json:"id"`
	Ok       bool          `json:"ok"`
	Error    string        `json:"error,omitempty"`
	Powered  *bool         `json:"powered,omitempty"`
	SSID     string        `json:"ssid,omitempty"`
	Saved    int           `json:"saved,omitempty"`
	Services []wifiService `json:"services,omitempty"`
}

type wifiService struct {
	Path        string   `json:"path"`
	Name        string   `json:"name"`
	State       string   `json:"state"`
	Strength    uint8    `json:"strength"`
	Security    []string `json:"security"`
	Favorite    bool     `json:"favorite"`
	AutoConnect bool     `json:"autoconnect"`
	Type        string   `json:"type"`
}

func main() {
	os.Remove(sockPath)

	ln, err := net.Listen("unix", sockPath)
	if err != nil {
		log.Fatalf("listen: %v", err)
	}
	defer ln.Close()

	if err := os.Chmod(sockPath, 0666); err != nil {
		log.Fatalf("chmod: %v", err)
	}

	client, err := wifi.NewClient()
	if err != nil {
		log.Fatalf("connect to connman: %v", err)
	}
	defer client.Close()

	log.Printf("rex-wifid listening on %s", sockPath)

	// Snapshot credentials on startup (best-effort).
	if n, err := wifi.CaptureBatocera(); err == nil && n > 0 {
		log.Printf("captured %d network(s) on startup", n)
	}

	for {
		conn, err := ln.Accept()
		if err != nil {
			log.Printf("accept: %v", err)
			continue
		}
		go handle(conn, client)
	}
}

func handle(conn net.Conn, client *wifi.Client) {
	defer conn.Close()
	sc := bufio.NewScanner(conn)
	for sc.Scan() {
		var req request
		if err := json.Unmarshal(sc.Bytes(), &req); err != nil {
			write(conn, response{ID: req.ID, Error: fmt.Sprintf("bad request: %v", err)})
			continue
		}
		write(conn, dispatch(client, req))
	}
}

func dispatch(c *wifi.Client, req request) response {
	switch req.Cmd {
	case "ping":
		return response{ID: req.ID, Ok: true}

	case "powered":
		on, err := c.WifiPowered()
		if err != nil {
			return response{ID: req.ID, Error: fmt.Sprintf("powered: %v", err)}
		}
		return response{ID: req.ID, Ok: true, Powered: &on}

	case "set-powered":
		if req.On == nil {
			return response{ID: req.ID, Error: "missing 'on' field"}
		}
		if err := c.SetWifiPowered(*req.On); err != nil {
			return response{ID: req.ID, Error: fmt.Sprintf("set-powered: %v", err)}
		}
		return response{ID: req.ID, Ok: true}

	case "scan":
		if err := c.ScanWifi(); err != nil {
			return response{ID: req.ID, Error: fmt.Sprintf("scan: %v", err)}
		}
		return response{ID: req.ID, Ok: true}

	case "services":
		svcs, err := c.Wifi()
		if err != nil {
			return response{ID: req.ID, Error: fmt.Sprintf("services: %v", err)}
		}
		out := make([]wifiService, len(svcs))
		for i, s := range svcs {
			out[i] = wifiService{
				Path:        string(s.Path),
				Name:        s.Name,
				State:       s.State,
				Strength:    s.Strength,
				Security:    s.Security,
				Favorite:    s.Favorite,
				AutoConnect: s.AutoConnect,
				Type:        s.Type,
			}
		}
		return response{ID: req.ID, Ok: true, Services: out}

	case "connect":
		if req.Path == "" {
			return response{ID: req.ID, Error: "missing 'path' field"}
		}
		var err error
		if req.Passphrase != "" {
			err = c.ConnectWithPassphrase(dbus.ObjectPath(req.Path), req.Passphrase)
		} else {
			err = c.Connect(dbus.ObjectPath(req.Path))
		}
		if err != nil {
			return response{ID: req.ID, Error: fmt.Sprintf("connect: %v", err)}
		}
		return response{ID: req.ID, Ok: true}

	case "disconnect":
		if req.Path == "" {
			return response{ID: req.ID, Error: "missing 'path' field"}
		}
		if err := c.Disconnect(dbus.ObjectPath(req.Path)); err != nil {
			return response{ID: req.ID, Error: fmt.Sprintf("disconnect: %v", err)}
		}
		return response{ID: req.ID, Ok: true}

	case "remove":
		if req.Path == "" || req.SSID == "" {
			return response{ID: req.ID, Error: "missing 'path' or 'ssid' field"}
		}
		wifi.Forget(req.SSID)
		if err := c.Remove(dbus.ObjectPath(req.Path)); err != nil {
			return response{ID: req.ID, Error: fmt.Sprintf("remove: %v", err)}
		}
		return response{ID: req.ID, Ok: true}

	case "capture":
		n, err := wifi.CaptureBatocera()
		if err != nil {
			return response{ID: req.ID, Error: fmt.Sprintf("capture: %v", err)}
		}
		return response{ID: req.ID, Ok: true, Saved: n}

	case "status":
		// Quick status for tmux bar: returns connected SSID or empty.
		svcs, err := c.Wifi()
		if err != nil {
			return response{ID: req.ID, Ok: false, Error: fmt.Sprintf("status: %v", err)}
		}
		for _, s := range svcs {
			if s.State == "online" || s.State == "ready" {
				return response{ID: req.ID, Ok: true, SSID: s.Name}
			}
		}
		return response{ID: req.ID, Ok: true, SSID: ""}

	default:
		return response{ID: req.ID, Error: fmt.Sprintf("unknown command %q", req.Cmd)}
	}
}

func write(conn net.Conn, resp response) {
	b, _ := json.Marshal(resp)
	b = append(b, '\n')
	conn.Write(b)
}
