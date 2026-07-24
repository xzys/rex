package wifi

import (
	"bufio"
	"encoding/json"
	"fmt"
	"net"

	"github.com/godbus/dbus/v5"
)

// Dial connects to the rex-wifid daemon over the Unix socket. The daemon runs
// on the Knulli host (as root); the socket at /tmp/rex-wifi.sock is visible
// inside the Debian chroot via /tmp bind mount.
func Dial() (*RPCClient, error) {
	conn, err := net.Dial("unix", "/tmp/rex-wifi.sock")
	if err != nil {
		return nil, fmt.Errorf("dial rex-wifid: %w (is the daemon running?)", err)
	}
	return &RPCClient{conn: conn, r: bufio.NewReader(conn)}, nil
}

// RPCClient is a WiFi client that speaks to rex-wifid over a Unix socket
// instead of talking to ConnMan over D-Bus directly. It implements the same
// interface as Client (the D-Bus client) for drop-in replacement.
type RPCClient struct {
	conn net.Conn
	r    *bufio.Reader
	seq  int // auto-incrementing request id
}

func (c *RPCClient) Close() error {
	return c.conn.Close()
}

type rpcReq struct {
	ID         string `json:"id"`
	Cmd        string `json:"cmd"`
	On         *bool  `json:"on,omitempty"`
	Path       string `json:"path,omitempty"`
	Passphrase string `json:"passphrase,omitempty"`
	SSID       string `json:"ssid,omitempty"`
}

type rpcResp struct {
	ID       string        `json:"id"`
	Ok       bool          `json:"ok"`
	Error    string        `json:"error,omitempty"`
	Powered  *bool         `json:"powered,omitempty"`
	SSID     string        `json:"ssid,omitempty"`
	Saved    int           `json:"saved,omitempty"`
	Services []rpcService  `json:"services,omitempty"`
}

type rpcService struct {
	Path        string   `json:"path"`
	Name        string   `json:"name"`
	State       string   `json:"state"`
	Strength    uint8    `json:"strength"`
	Security    []string `json:"security"`
	Favorite    bool     `json:"favorite"`
	AutoConnect bool     `json:"autoconnect"`
	Type        string   `json:"type"`
}

func (c *RPCClient) call(req rpcReq) (rpcResp, error) {
	c.seq++
	if req.ID == "" {
		req.ID = fmt.Sprintf("%d", c.seq)
	}
	b, err := json.Marshal(req)
	if err != nil {
		return rpcResp{}, fmt.Errorf("marshal: %w", err)
	}
	b = append(b, '\n')
	if _, err := c.conn.Write(b); err != nil {
		return rpcResp{}, fmt.Errorf("write: %w", err)
	}
	line, err := c.r.ReadBytes('\n')
	if err != nil {
		return rpcResp{}, fmt.Errorf("read: %w", err)
	}
	var resp rpcResp
	if err := json.Unmarshal(line, &resp); err != nil {
		return rpcResp{}, fmt.Errorf("unmarshal response: %w", err)
	}
	if !resp.Ok {
		return resp, fmt.Errorf("%s", resp.Error)
	}
	return resp, nil
}

// WifiPowered reports whether the WiFi technology is Powered.
func (c *RPCClient) WifiPowered() (bool, error) {
	resp, err := c.call(rpcReq{Cmd: "powered"})
	if err != nil {
		return false, fmt.Errorf("GetProperties: %w", err)
	}
	if resp.Powered == nil {
		return false, fmt.Errorf("GetProperties: missing powered field")
	}
	return *resp.Powered, nil
}

// SetWifiPowered toggles the WiFi technology on/off.
func (c *RPCClient) SetWifiPowered(on bool) error {
	_, err := c.call(rpcReq{Cmd: "set-powered", On: &on})
	return err
}

// ScanWifi triggers a WiFi scan.
func (c *RPCClient) ScanWifi() error {
	_, err := c.call(rpcReq{Cmd: "scan"})
	return err
}

// Services returns all visible WiFi services.
func (c *RPCClient) Services() ([]Service, error) {
	resp, err := c.call(rpcReq{Cmd: "services"})
	if err != nil {
		return nil, fmt.Errorf("GetServices: %w", err)
	}
	out := make([]Service, len(resp.Services))
	for i, s := range resp.Services {
		out[i] = Service{
			Path:        dbus.ObjectPath(s.Path),
			Name:        s.Name,
			State:       s.State,
			Strength:    s.Strength,
			Security:    s.Security,
			Favorite:    s.Favorite,
			AutoConnect: s.AutoConnect,
			Type:        s.Type,
		}
	}
	return out, nil
}

// Wifi filters visible services to WiFi type only, sorted by
// connected state, then favorite status, then signal strength.
func (c *RPCClient) Wifi() ([]Service, error) {
	all, err := c.Services()
	if err != nil {
		return nil, err
	}
	return filterWifi(all), nil
}

// Connect connects to the given service path. Use ConnectWithPassphrase for
// secure networks.
func (c *RPCClient) Connect(path dbus.ObjectPath) error {
	_, err := c.call(rpcReq{Cmd: "connect", Path: string(path)})
	return err
}

// ConnectWithPassphrase connects with credentials via the ConnMan agent.
func (c *RPCClient) ConnectWithPassphrase(path dbus.ObjectPath, passphrase string) error {
	_, err := c.call(rpcReq{Cmd: "connect", Path: string(path), Passphrase: passphrase})
	return err
}

// Disconnect disconnects from the given service.
func (c *RPCClient) Disconnect(path dbus.ObjectPath) error {
	_, err := c.call(rpcReq{Cmd: "disconnect", Path: string(path)})
	return err
}

// Remove forgets a service and removes it from persistent config.
func (c *RPCClient) Remove(path dbus.ObjectPath) error {
	svc, err := c.serviceByPath(path)
	if err != nil {
		return err
	}
	ssid := ""
	if svc != nil {
		ssid = svc.Name
	}
	// Best-effort forget.
	if ssid != "" {
		Forget(ssid)
	}
	_, err = c.call(rpcReq{Cmd: "remove", Path: string(path), SSID: ssid})
	return err
}

func (c *RPCClient) serviceByPath(path dbus.ObjectPath) (*Service, error) {
	svcs, err := c.Services()
	if err != nil {
		return nil, err
	}
	for i := range svcs {
		if svcs[i].Path == path {
			return &svcs[i], nil
		}
	}
	return nil, nil
}

// StatusSSID returns the SSID of the currently connected WiFi network, or
// empty string if not connected. Used by the tmux status bar.
func (c *RPCClient) StatusSSID() (string, error) {
	resp, err := c.call(rpcReq{Cmd: "status"})
	if err != nil {
		return "", err
	}
	return resp.SSID, nil
}
