package security

import (
	"encoding/json"
	"errors"
	"log"
	"net"
	"os"
	"slices"
	"strings"
)

type Firewall struct {
	Mode       FirewallMode `json:"mode"`
	BlockedIps []string     `json:"blockedIps"`
	AllowedIps []string     `json:"allowedIps"`
}
type FirewallMode int

const (
	ModeUnknown FirewallMode = iota
	ModeWhiteList
	ModeBlackList
)

type IFireWall interface {
	FirewallFilter(ip string) error
}

func (m *FirewallMode) UnmarshalJSON(b []byte) error {
	var s string
	if err := json.Unmarshal(b, &s); err != nil {
		return err
	}

	switch strings.ToLower(s) {
	case "whitelist":
		*m = ModeWhiteList
	case "blacklist":
		*m = ModeBlackList
	default:
		*m = ModeUnknown
	}

	return nil
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
func NewFirewall(blockedIps []string, allowedIps []string) IFireWall {
	return &Firewall{BlockedIps: blockedIps, AllowedIps: allowedIps}
}

var ErrIpNotAllowed = errors.New("IP not allowed")

func (f *Firewall) FirewallFilter(ip string) error {
	ip = normalizeToIPv4(ip)
	switch f.Mode {
	default: // Default to whitelist rules
		if !f.isAllowed(ip) {
			return errors.New("Access denied")
		}
	case ModeWhiteList:
		if !f.isAllowed(ip) {
			return errors.New("Access denied")
		}
	case ModeBlackList:
		if f.isBlocked(ip) {
			return errors.New("Access denied")
		}

	}
	return nil

}
func (f *Firewall) isBlocked(ip string) bool {
	return slices.Contains(f.BlockedIps, ip)
}

func (f *Firewall) isAllowed(ip string) bool {
	return slices.Contains(f.AllowedIps, ip)
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

	// Convert IPv6 loopback to IPv4 loopback
	if parsed.Equal(net.IPv6loopback) {
		return "127.0.0.1"
	}

	// Otherwise return normalized IPv6
	return parsed.String()
}
