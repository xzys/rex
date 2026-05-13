package wifi

import (
	"crypto/sha1"
	"encoding/hex"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
)

// ConfigPath is ConnMan's provisioning config that survives reboots on Brick.
// /var/lib/connman/batocera-custom.config is a symlink to this path baked into
// the rootfs; ConnMan reads .config files at startup and creates Immutable
// services from them, which means Autoconnect=true is honored on every boot.
const ConfigPath = "/userdata/system/network-connman.config"

// Profile is one saved wifi network.
type Profile struct {
	SSID       string
	Passphrase string
	Hidden     bool
}

// LoadProfiles parses ConfigPath into profiles keyed by SSID. Missing file is
// not an error — returns an empty map.
func LoadProfiles() (map[string]Profile, error) { return loadProfilesAt(ConfigPath) }

// Remember writes (or replaces) a profile in ConfigPath. Returns true if the
// file was changed.
func Remember(ssid, passphrase string) (bool, error) {
	return rememberAt(ConfigPath, ssid, passphrase)
}

// Forget removes a profile from ConfigPath. Returns true if a profile was
// removed. Missing file or unknown SSID is a no-op.
func Forget(ssid string) (bool, error) { return forgetAt(ConfigPath, ssid) }

func loadProfilesAt(path string) (map[string]Profile, error) {
	out := map[string]Profile{}
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return out, nil
		}
		return nil, err
	}
	var cur *Profile
	for line := range strings.SplitSeq(string(data), "\n") {
		t := strings.TrimSpace(line)
		if t == "" || strings.HasPrefix(t, "#") {
			continue
		}
		if strings.HasPrefix(t, "[") && strings.HasSuffix(t, "]") {
			if cur != nil && cur.SSID != "" {
				out[cur.SSID] = *cur
			}
			if strings.HasPrefix(t, "[service_") {
				cur = &Profile{}
			} else {
				cur = nil
			}
			continue
		}
		if cur == nil {
			continue
		}
		eq := strings.IndexByte(t, '=')
		if eq <= 0 {
			continue
		}
		k := strings.TrimSpace(t[:eq])
		v := strings.TrimSpace(t[eq+1:])
		switch k {
		case "Name":
			cur.SSID = v
		case "Passphrase":
			cur.Passphrase = v
		case "Hidden":
			cur.Hidden = strings.EqualFold(v, "true")
		}
	}
	if cur != nil && cur.SSID != "" {
		out[cur.SSID] = *cur
	}
	return out, nil
}

func rememberAt(path, ssid, passphrase string) (bool, error) {
	if ssid == "" {
		return false, fmt.Errorf("empty SSID")
	}
	profiles, err := loadProfilesAt(path)
	if err != nil {
		return false, err
	}
	if existing, ok := profiles[ssid]; ok && existing.Passphrase == passphrase {
		return false, nil
	}
	profiles[ssid] = Profile{SSID: ssid, Passphrase: passphrase}
	return true, writeProfilesAt(path, profiles)
}

func forgetAt(path, ssid string) (bool, error) {
	profiles, err := loadProfilesAt(path)
	if err != nil {
		return false, err
	}
	if _, ok := profiles[ssid]; !ok {
		return false, nil
	}
	delete(profiles, ssid)
	return true, writeProfilesAt(path, profiles)
}

func writeProfilesAt(path string, profiles map[string]Profile) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	var b strings.Builder
	b.WriteString("# Managed by rex wifi. Networks listed here are provisioned\n")
	b.WriteString("# into ConnMan on every boot via /var/lib/connman/batocera-custom.config.\n\n")
	b.WriteString("[global]\nName=rex-saved\n\n")
	ssids := make([]string, 0, len(profiles))
	for s := range profiles {
		ssids = append(ssids, s)
	}
	sort.Strings(ssids)
	for _, ssid := range ssids {
		p := profiles[ssid]
		fmt.Fprintf(&b, "[service_rex_%s]\n", serviceSlug(p.SSID))
		b.WriteString("Type=wifi\n")
		fmt.Fprintf(&b, "Name=%s\n", p.SSID)
		if p.Hidden {
			b.WriteString("Hidden=true\n")
		}
		if p.Passphrase != "" {
			fmt.Fprintf(&b, "Passphrase=%s\n", p.Passphrase)
		}
		b.WriteString("Autoconnect=true\n\n")
	}
	tmp := path + ".tmp"
	if err := os.WriteFile(tmp, []byte(b.String()), 0o600); err != nil {
		return err
	}
	return os.Rename(tmp, path)
}

var slugRe = regexp.MustCompile(`[^a-z0-9]+`)

// serviceSlug returns a stable, [service_…]-safe identifier for an SSID.
// Format is `<sanitized>_<hash>` so two SSIDs that sanitize the same still
// get distinct service sections.
func serviceSlug(ssid string) string {
	low := strings.ToLower(ssid)
	clean := slugRe.ReplaceAllString(low, "_")
	clean = strings.Trim(clean, "_")
	if clean == "" {
		clean = "x"
	}
	sum := sha1.Sum([]byte(ssid))
	return clean + "_" + hex.EncodeToString(sum[:4])
}
