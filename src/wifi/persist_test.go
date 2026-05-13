package wifi

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestRememberRoundTrip(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "network-connman.config")

	if changed, err := rememberAt(path, "Home_5G", "correct horse"); err != nil || !changed {
		t.Fatalf("first remember: changed=%v err=%v", changed, err)
	}
	if changed, err := rememberAt(path, "Home_5G", "correct horse"); err != nil || changed {
		t.Fatalf("idempotent remember should report unchanged: changed=%v err=%v", changed, err)
	}
	if changed, err := rememberAt(path, "Home_5G", "new pass"); err != nil || !changed {
		t.Fatalf("passphrase change: changed=%v err=%v", changed, err)
	}

	profiles, err := loadProfilesAt(path)
	if err != nil {
		t.Fatal(err)
	}
	if got := profiles["Home_5G"].Passphrase; got != "new pass" {
		t.Fatalf("want new pass, got %q", got)
	}
}

func TestForget(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "network-connman.config")

	_, _ = rememberAt(path, "A", "a")
	_, _ = rememberAt(path, "B", "b")

	if changed, err := forgetAt(path, "A"); err != nil || !changed {
		t.Fatalf("forget A: changed=%v err=%v", changed, err)
	}
	if changed, err := forgetAt(path, "A"); err != nil || changed {
		t.Fatalf("re-forget A: changed=%v err=%v", changed, err)
	}
	profiles, _ := loadProfilesAt(path)
	if _, ok := profiles["A"]; ok {
		t.Fatal("A still present")
	}
	if _, ok := profiles["B"]; !ok {
		t.Fatal("B unexpectedly gone")
	}
}

func TestOpenNetworkOmitsPassphraseLine(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "network-connman.config")
	if _, err := rememberAt(path, "Cafe", ""); err != nil {
		t.Fatal(err)
	}
	data, _ := os.ReadFile(path)
	if strings.Contains(string(data), "Passphrase=") {
		t.Fatalf("open network should not emit Passphrase= line:\n%s", data)
	}
	if !strings.Contains(string(data), "Name=Cafe") {
		t.Fatalf("Name= line missing:\n%s", data)
	}
}

func TestCaptureBatocera(t *testing.T) {
	dir := t.TempDir()
	batocera := filepath.Join(dir, "batocera.conf")
	cfg := filepath.Join(dir, "network-connman.config")

	conf := `# comment line
wifi.enabled=1
wifi.country=US
wifi.ssid=Home_5G
wifi.key=correct horse battery
wifi2.ssid=Coffee Shop
wifi2.key=
wifi3.ssid=
wifi3.key=ignored
wifi.adhoc.ssid=NETPLAY_AP
wifi.adhoc.key=12345678
wifi.hidden.ssid=Stealth
wifi.hidden.key=secret
`
	if err := os.WriteFile(batocera, []byte(conf), 0o600); err != nil {
		t.Fatal(err)
	}
	n, err := captureAt(batocera, cfg)
	if err != nil {
		t.Fatal(err)
	}
	if n != 2 {
		t.Fatalf("expected 2 captures (Home_5G, Coffee Shop), got %d", n)
	}
	profiles, _ := loadProfilesAt(cfg)
	if profiles["Home_5G"].Passphrase != "correct horse battery" {
		t.Fatalf("Home_5G passphrase wrong: %q", profiles["Home_5G"].Passphrase)
	}
	if _, ok := profiles["Coffee Shop"]; !ok {
		t.Fatal("Coffee Shop missing")
	}
	if profiles["Coffee Shop"].Passphrase != "" {
		t.Fatalf("Coffee Shop should be open, got %q", profiles["Coffee Shop"].Passphrase)
	}
	if _, ok := profiles["NETPLAY_AP"]; ok {
		t.Fatal("adhoc must not be captured")
	}
	if _, ok := profiles["Stealth"]; ok {
		t.Fatal("hidden must not be captured (separate slot, not implemented)")
	}
	// Re-running with no changes should report 0
	n2, err := captureAt(batocera, cfg)
	if err != nil || n2 != 0 {
		t.Fatalf("idempotent capture: n=%d err=%v", n2, err)
	}
	// Changing a key bumps the count
	conf2 := strings.Replace(conf, "correct horse battery", "rotated", 1)
	_ = os.WriteFile(batocera, []byte(conf2), 0o600)
	n3, err := captureAt(batocera, cfg)
	if err != nil || n3 != 1 {
		t.Fatalf("rotated capture: n=%d err=%v", n3, err)
	}
}

func TestServiceSlugStable(t *testing.T) {
	s1 := serviceSlug("Verizon_P4YQMX")
	s2 := serviceSlug("Verizon_P4YQMX")
	if s1 != s2 {
		t.Fatalf("slug not stable: %q vs %q", s1, s2)
	}
	if serviceSlug("A") == serviceSlug("B") {
		t.Fatal("distinct SSIDs collided")
	}
	if got := serviceSlug("  "); got == "" {
		t.Fatal("empty-ish SSID produced empty slug")
	}
}
