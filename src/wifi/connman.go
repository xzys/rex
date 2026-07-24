package wifi

import (
	"errors"
	"fmt"
	"sort"

	"github.com/godbus/dbus/v5"
)

// ErrNotAvailable is returned by WifiPowered / SetWifiPowered when the WiFi
// technology object is not registered with ConnMan. Callers should treat this
// as a transient condition — the XRADIO driver on the Brick can deregister
// the interface, and it typically recovers on its own within a few seconds.
var ErrNotAvailable = errors.New("WiFi not available")

// ErrBlocked is returned when D-Bus policy rejects the message (e.g. the
// connman-rex.conf drop-in is missing from /etc/dbus-1/system.d/ on a fresh
// boot). Unlike ErrNotAvailable this does not self-recover.
var ErrBlocked = errors.New("WiFi blocked by D-Bus policy")

// classifyDBusError maps a D-Bus error to a user-friendly sentinel. Other
// errors pass through unchanged.
func classifyDBusError(err error) error {
	if err == nil {
		return nil
	}
	var derr dbus.Error
	if !errors.As(err, &derr) {
		return err
	}
	switch derr.Name {
	case "org.freedesktop.DBus.Error.UnknownObject",
		"org.freedesktop.DBus.Error.UnknownMethod":
		return ErrNotAvailable
	case "org.freedesktop.DBus.Error.AccessDenied":
		return ErrBlocked
	}
	return err
}

const (
	busName       = "net.connman"
	managerIface  = "net.connman.Manager"
	serviceIface  = "net.connman.Service"
	techIface     = "net.connman.Technology"
	wifiTechPath  = "/net/connman/technology/wifi"
)

type Service struct {
	Path        dbus.ObjectPath
	Name        string
	State       string
	Strength    uint8
	Security    []string
	Favorite    bool
	AutoConnect bool
	Type        string
}

type Client struct {
	conn *dbus.Conn
}

func NewClient() (*Client, error) {
	conn, err := dbus.ConnectSystemBus()
	if err != nil {
		return nil, fmt.Errorf("connect system bus: %w", err)
	}
	return &Client{conn: conn}, nil
}

func (c *Client) Close() error {
	return c.conn.Close()
}

func (c *Client) Services() ([]Service, error) {
	obj := c.conn.Object(busName, "/")
	var raw []struct {
		Path  dbus.ObjectPath
		Props map[string]dbus.Variant
	}
	if err := obj.Call(managerIface+".GetServices", 0).Store(&raw); err != nil {
		return nil, fmt.Errorf("GetServices: %w", err)
	}

	out := make([]Service, 0, len(raw))
	for _, r := range raw {
		s := Service{Path: r.Path}
		if v, ok := r.Props["Name"]; ok {
			_ = v.Store(&s.Name)
		}
		if v, ok := r.Props["State"]; ok {
			_ = v.Store(&s.State)
		}
		if v, ok := r.Props["Strength"]; ok {
			_ = v.Store(&s.Strength)
		}
		if v, ok := r.Props["Security"]; ok {
			_ = v.Store(&s.Security)
		}
		if v, ok := r.Props["Favorite"]; ok {
			_ = v.Store(&s.Favorite)
		}
		if v, ok := r.Props["AutoConnect"]; ok {
			_ = v.Store(&s.AutoConnect)
		}
		if v, ok := r.Props["Type"]; ok {
			_ = v.Store(&s.Type)
		}
		out = append(out, s)
	}
	return out, nil
}

func (c *Client) Wifi() ([]Service, error) {
	all, err := c.Services()
	if err != nil {
		return nil, err
	}
	out := all[:0]
	for _, s := range all {
		if s.Type == "wifi" {
			out = append(out, s)
		}
	}
	sort.SliceStable(out, func(i, j int) bool {
		// connected first, then favorites, then by signal
		ci, cj := isConnected(out[i].State), isConnected(out[j].State)
		if ci != cj {
			return ci
		}
		if out[i].Favorite != out[j].Favorite {
			return out[i].Favorite
		}
		return out[i].Strength > out[j].Strength
	})
	return out, nil
}

func isConnected(state string) bool {
	return state == "online" || state == "ready"
}

// ScanWifi triggers a WiFi scan. Returns ErrNotAvailable if the WiFi
// technology is not registered.
func (c *Client) ScanWifi() error {
	obj := c.conn.Object(busName, wifiTechPath)
	return classifyDBusError(obj.Call(techIface+".Scan", 0).Err)
}

// WifiPowered reads the WiFi technology's Powered property.
// Returns ErrNotAvailable if the WiFi technology is not registered with
// ConnMan (e.g. driver flaked), or ErrBlocked if the D-Bus policy rejects
// the call.
func (c *Client) WifiPowered() (bool, error) {
	obj := c.conn.Object(busName, wifiTechPath)
	var props map[string]dbus.Variant
	if err := obj.Call(techIface+".GetProperties", 0).Store(&props); err != nil {
		return false, classifyDBusError(fmt.Errorf("GetProperties: %w", err))
	}
	v, ok := props["Powered"]
	if !ok {
		return false, fmt.Errorf("Powered property missing")
	}
	var on bool
	if err := v.Store(&on); err != nil {
		return false, fmt.Errorf("decode Powered: %w", err)
	}
	return on, nil
}

// SetWifiPowered toggles the WiFi technology on/off via ConnMan.
// Returns ErrNotAvailable if the WiFi technology is not registered, or
// ErrBlocked if the D-Bus policy rejects the call.
func (c *Client) SetWifiPowered(on bool) error {
	obj := c.conn.Object(busName, wifiTechPath)
	err := obj.Call(techIface+".SetProperty", 0, "Powered", dbus.MakeVariant(on)).Err
	return classifyDBusError(err)
}

func (c *Client) Connect(path dbus.ObjectPath) error {
	obj := c.conn.Object(busName, path)
	// Long timeout: ConnMan blocks here for the entire association/DHCP flow.
	return obj.Call(serviceIface+".Connect", 0).Err
}

func (c *Client) Disconnect(path dbus.ObjectPath) error {
	obj := c.conn.Object(busName, path)
	return obj.Call(serviceIface+".Disconnect", 0).Err
}

func (c *Client) Remove(path dbus.ObjectPath) error {
	obj := c.conn.Object(busName, path)
	return obj.Call(serviceIface+".Remove", 0).Err
}
