package wifi

import (
	"fmt"
	"log/slog"
	"sync"

	"github.com/godbus/dbus/v5"
)

// agentPath is the D-Bus object path under which we export the ConnMan
// Agent. ConnMan only requires the path be unique on our bus name; the
// exact value doesn't matter.
const agentPath = dbus.ObjectPath("/rex/wifi/agent")

const agentIface = "net.connman.Agent"

// passphraseAgent implements net.connman.Agent. It hands back a single
// passphrase for the in-flight Connect call. Use newPassphraseAgent +
// (*Client).RegisterAgent / UnregisterAgent.
type passphraseAgent struct {
	mu         sync.Mutex
	passphrase string
}

// RequestInput is called by ConnMan when a service needs credentials.
// `fields` describes what's required (Passphrase, Name, Identity, …).
// For PSK networks ConnMan asks for `Passphrase`; that's all we handle.
func (a *passphraseAgent) RequestInput(svc dbus.ObjectPath, fields map[string]dbus.Variant) (map[string]dbus.Variant, *dbus.Error) {
	slog.Info("connman requested input", "service", svc, "fields", fieldNames(fields))
	a.mu.Lock()
	pw := a.passphrase
	a.mu.Unlock()
	if pw == "" {
		return nil, dbus.NewError("net.connman.Agent.Error.Canceled", []any{"no passphrase available"})
	}
	out := map[string]dbus.Variant{}
	if _, ok := fields["Passphrase"]; ok {
		out["Passphrase"] = dbus.MakeVariant(pw)
	}
	return out, nil
}

// ReportError is called when a Connect call fails after our agent already
// supplied input. Returning the magic "net.connman.Agent.Error.Retry"
// error tells ConnMan to call RequestInput again — we don't, since the
// passphrase we have was wrong and retrying with the same value loops.
func (a *passphraseAgent) ReportError(svc dbus.ObjectPath, errStr string) *dbus.Error {
	slog.Warn("connman reported connect error", "service", svc, "err", errStr)
	return nil
}

func (a *passphraseAgent) Cancel() *dbus.Error  { return nil }
func (a *passphraseAgent) Release() *dbus.Error { return nil }

func fieldNames(fields map[string]dbus.Variant) []string {
	out := make([]string, 0, len(fields))
	for k := range fields {
		out = append(out, k)
	}
	return out
}

// ConnectWithPassphrase connects to `path` while supplying `passphrase`
// to ConnMan via a transient Agent. Blocks until ConnMan finishes the
// association/DHCP flow (or errors out).
func (c *Client) ConnectWithPassphrase(path dbus.ObjectPath, passphrase string) error {
	agent := &passphraseAgent{passphrase: passphrase}
	if err := c.registerAgent(agent); err != nil {
		return fmt.Errorf("register agent: %w", err)
	}
	defer func() {
		if err := c.unregisterAgent(); err != nil {
			slog.Warn("unregister agent failed", "err", err)
		}
	}()
	return c.Connect(path)
}

func (c *Client) registerAgent(a *passphraseAgent) error {
	if err := c.conn.Export(a, agentPath, agentIface); err != nil {
		return fmt.Errorf("export agent: %w", err)
	}
	obj := c.conn.Object(busName, "/")
	return obj.Call(managerIface+".RegisterAgent", 0, agentPath).Err
}

func (c *Client) unregisterAgent() error {
	obj := c.conn.Object(busName, "/")
	call := obj.Call(managerIface+".UnregisterAgent", 0, agentPath)
	// Best effort: unexport even if Unregister fails (e.g. ConnMan already
	// dropped the agent).
	_ = c.conn.Export(nil, agentPath, agentIface)
	return call.Err
}
