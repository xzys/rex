package menu

import (
	"bufio"
	"os"
	"path/filepath"
	"strings"
)

type SSHHost struct {
	Name         string
	User         string
	Host         string
	Port         string
	IdentityFile string
}

func GetSSHHosts(configHosts []string) ([]SSHHost, error) {
	allHosts, err := parseSSHConfig()
	if err != nil {
		return nil, err
	}

	if len(configHosts) > 0 {
		hostMap := make(map[string]SSHHost)
		for _, host := range allHosts {
			hostMap[host.Name] = host
		}

		var filtered []SSHHost
		for _, name := range configHosts {
			if host, ok := hostMap[name]; ok {
				filtered = append(filtered, host)
			}
		}
		return filtered, nil
	}

	return allHosts, nil
}

func parseSSHConfig() ([]SSHHost, error) {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return nil, err
	}

	configPath := filepath.Join(homeDir, ".ssh", "config")
	file, err := os.Open(configPath)
	if err != nil {
		return nil, err
	}
	defer file.Close()

	var hosts []SSHHost
	scanner := bufio.NewScanner(file)

	var currentHost *SSHHost
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}

		lowerLine := strings.ToLower(line)
		if strings.HasPrefix(lowerLine, "host ") {
			if currentHost != nil && currentHost.Name != "" {
				hosts = append(hosts, *currentHost)
			}
			hostName := strings.TrimSpace(strings.TrimPrefix(line, line[:5]))
			if hostName != "*" && !strings.Contains(hostName, "*") {
				currentHost = &SSHHost{
					Name: hostName,
					Port: "22",
				}
			} else {
				currentHost = nil
			}
		} else if currentHost != nil {
			parts := strings.Fields(line)
			if len(parts) >= 2 {
				key := strings.ToLower(parts[0])
				value := strings.Join(parts[1:], " ")
				switch key {
				case "user":
					currentHost.User = value
				case "hostname":
					currentHost.Host = value
				case "port":
					currentHost.Port = value
				case "identityfile":
					currentHost.IdentityFile = value
				}
			}
		}
	}

	if currentHost != nil && currentHost.Name != "" {
		hosts = append(hosts, *currentHost)
	}

	return hosts, scanner.Err()
}
