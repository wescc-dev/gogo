package security

import (
	"encoding/json"
	"fmt"
	"log"
	"net"
	"os"
	"strconv"
	"strings"
)

type Firewall struct {
	Enabled    bool         `json:"enabled"`
	Mode       FirewallMode `json:"mode"`
	BlockedIps []string     `json:"blockedIps"`
	AllowedIps []string     `json:"allowedIps"`
}

type IFireWall interface {
	FirewallFilter(ip string) error
}

func NewFireWall(path string) (IFireWall, error) {
	// Read entire file into memory
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var fw Firewall
	err = json.Unmarshal(data, &fw)
	if err != nil {
		log.Fatal(err)
	}
	return &fw, nil
}

func (f *Firewall) FirewallFilter(ip string) error {
	if !f.Enabled {
		log.Println("Firewall is disabled. Access granted.")
		return nil
	}
	switch f.Mode {
	default: // Default to whitelist rules
		if !f.isAllowed(ip) {
			return fmt.Errorf("access denied. IP:%s; mode not set (default is whitelist)", ip)
		}
	case ModeWhiteList:
		if !f.isAllowed(ip) {
			return fmt.Errorf("access denied. IP:%s; mode:%s", ip, f.Mode)
		}
	case ModeBlackList:
		if f.isBlocked(ip) {
			return fmt.Errorf("access denied. IP:%s; mode:%s", ip, f.Mode)
		}

	}
	return nil

}
func (f *Firewall) isBlocked(ip string) bool {
	for _, rule := range f.BlockedIps {
		if ruleMatches(rule, ip) {
			return true
		}
	}
	return false
}

func (f *Firewall) isAllowed(ip string) bool {
	for _, rule := range f.AllowedIps {
		if ruleMatches(rule, ip) {
			return true
		}
	}
	return false
}

func ruleMatches(rule, ip string) bool {
	rule = strings.TrimSpace(rule)
	if rule == "" {
		return false
	}

	if strings.Contains(rule, "*") {
		if rule == "*" {
			return true
		}
		return wildcardIPv4Matches(rule, ip)
	}

	parsedIP := net.ParseIP(ip)
	if parsedIP == nil {
		return false
	}

	if _, network, err := net.ParseCIDR(rule); err == nil {
		return network.Contains(parsedIP)
	}

	parsedRule := net.ParseIP(rule)
	return parsedRule != nil && parsedRule.Equal(parsedIP)
}

func wildcardIPv4Matches(rule, ip string) bool {
	ruleParts := strings.Split(rule, ".")
	parsedIP := net.ParseIP(ip)
	if len(ruleParts) != net.IPv4len || parsedIP == nil || parsedIP.To4() == nil {
		return false
	}
	ipBytes := parsedIP.To4()

	for i, part := range ruleParts {
		if part == "*" {
			continue
		}
		ruleOctet, err := strconv.Atoi(part)
		if err != nil || ruleOctet < 0 || ruleOctet > 255 || ruleOctet != int(ipBytes[i]) {
			return false
		}
	}
	return true
}

func normalizeToIPv4(ip string) string {
	parsed := net.ParseIP(ip)
	if parsed == nil {
		return ip
	}

	// Convert IPv6-mapped IPv4 (::ffff:x.x.x.x) to IPv4
	if v4 := parsed.To4(); v4 != nil {
		return v4.String()
	}

	// Otherwise return normalized IPv6
	return parsed.String()
}
