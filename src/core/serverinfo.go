package core

import "time"

type ServerInfo struct {
	Title                   string
	HostName                string
	Port                    string
	TLSEnabled              bool
	Uptime                  time.Duration
	StartTime               time.Time
	CurrentConnections      int
	TotalConnections        int
	OS                      string
	Architecture            string
	NumCpus                 int
	GopherRoot              string
	GophermapTemplateName   string
	ServerSoftwareName      string
	ServerSoftwareVersion   string
	ServerSoftwareCopyright string
	ServerSoftwareLicense   string
}

type IServerInfoProvider interface {
	GetCurrentServerInfo() ServerInfo
}
