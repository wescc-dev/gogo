package security

import (
	"encoding/json"
	"strings"
)

type FirewallMode int

const (
	ModeUnknown FirewallMode = iota
	ModeWhiteList
	ModeBlackList
)

func (m FirewallMode) String() string {
	switch m {
	case ModeWhiteList:
		return "whitelist"
	case ModeBlackList:
		return "blacklist"
	default:
		return "unknown"
	}
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
