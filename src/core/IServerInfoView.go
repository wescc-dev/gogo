package core

import "time"

type ServerInfoView struct {
	GopherHoleName          string
	HostName                string
	Port                    string
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

type IServerInfoViewProvider interface {
	GetCurrentServerInfo() ServerInfoView
}
