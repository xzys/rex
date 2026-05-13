package wifi

import (
	"bufio"
	"os"
	"strings"
)

// BatoceraConfPath is the persistent Batocera settings file that EmulationStation
// writes wifi credentials into. We mine it for SSIDs/passphrases the user has
// already entered (via ES) and forward them to ConfigPath so they survive across
// reboots even when ES later overwrites wifi.ssid with a different network.
const BatoceraConfPath = "/userdata/system/batocera.conf"

// CaptureBatocera scans BatoceraConfPath for wifi[N].ssid/wifi[N].key pairs and
// merges them into ConfigPath. Returns the number of newly-saved (or updated)
// profiles. Missing files are not errors.
func CaptureBatocera() (int, error) {
	return captureAt(BatoceraConfPath, ConfigPath)
}

func captureAt(batoceraPath, configPath string) (int, error) {
	pairs, err := readBatoceraWifiPairs(batoceraPath)
	if err != nil {
		return 0, err
	}
	saved := 0
	for ssid, key := range pairs {
		changed, err := rememberAt(configPath, ssid, key)
		if err != nil {
			return saved, err
		}
		if changed {
			saved++
		}
	}
	return saved, nil
}

// readBatoceraWifiPairs returns a map of SSID -> passphrase for every
// wifi*.ssid / wifi*.key pair found in the file. The 'adhoc' and 'hidden'
// variants are skipped — adhoc is a tethering AP, not a saved client network.
func readBatoceraWifiPairs(path string) (map[string]string, error) {
	f, err := os.Open(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}
	defer f.Close()

	ssids := map[string]string{} // slot suffix ("", "2", "3") -> ssid
	keys := map[string]string{}

	sc := bufio.NewScanner(f)
	for sc.Scan() {
		line := strings.TrimSpace(sc.Text())
		if line == "" || strings.HasPrefix(line, "#") || strings.HasPrefix(line, ";") {
			continue
		}
		eq := strings.IndexByte(line, '=')
		if eq <= 0 {
			continue
		}
		k := strings.TrimSpace(line[:eq])
		v := strings.TrimSpace(line[eq+1:])
		if !strings.HasPrefix(k, "wifi") {
			continue
		}
		rest := k[len("wifi"):]
		// Allow wifi.ssid, wifi2.ssid, wifi3.ssid; reject wifi.adhoc.* / wifi.hidden.*
		var slot, field string
		if dot := strings.IndexByte(rest, '.'); dot >= 0 {
			slot, field = rest[:dot], rest[dot+1:]
		} else {
			continue
		}
		if strings.ContainsRune(slot, '.') {
			continue
		}
		// slot is "" for wifi.X, or "2"/"3" for wifi2.X
		// adhoc/hidden show up as wifi.adhoc.ssid → field becomes "adhoc.ssid"
		if strings.Contains(field, ".") {
			continue
		}
		switch field {
		case "ssid":
			if v != "" {
				ssids[slot] = v
			}
		case "key":
			keys[slot] = v
		}
	}
	if err := sc.Err(); err != nil {
		return nil, err
	}

	out := map[string]string{}
	for slot, ssid := range ssids {
		out[ssid] = keys[slot] // key may be empty (open network)
	}
	return out, nil
}
