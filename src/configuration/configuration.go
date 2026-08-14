package configuration

import (
	"os"
	"runtime"
	"strconv"
	"time"

	"github.com/joho/godotenv"
)

const (
	AppName   = "Wes C's Go Gopher Server"
	Version   = "0.1.1"
	Copyright = "Copyright©️ 2026 Wes C"
	Link      = "https://github.com/wescc-dev/gogopher"
	Footer    = "i                   ------ Go Gopher Server© Wes C. -----\t\terror.host\t1\r\n"
)

type Configuration struct {
	Title                  string
	HostName               string
	BindAddress            string // In a Docker container, the ip will be different from the host ip
	Port                   string
	GopherRoot             string
	FireWallConfigFile     string
	GophermapTemplateName  string
	RequestTimeoutDuration time.Duration
	OS                     string
	Architecture           string
	NumCpus                int
}

var _configuration *Configuration = nil

func GetConfiguration() Configuration {
	var _ = godotenv.Load(".env")
	if _configuration == nil {
		var envRequestTimeoutSeconds = getEnv("READWRITE_TIMEOUT_SECONDS", "30")
		var requestTimeoutSeconds int
		if val, err := strconv.Atoi(envRequestTimeoutSeconds); err != nil {
			requestTimeoutSeconds = 30
		} else {
			requestTimeoutSeconds = val
		}
		_configuration = &Configuration{
			Title:                  getEnv("TITLE", "Wes C's Gopher Hole"),
			HostName:               getEnv("HOSTNAME", "localhost"),
			BindAddress:            getEnv("HOST_BIND_IP", "0.0.0.0"),
			Port:                   getEnv("PORT", "70"),
			GopherRoot:             getEnv("GOPHER_ROOT", "gopher-root"),
			FireWallConfigFile:     getEnv("FIREWALL_CONFIG_FILE", "firewall-config.json"),
			GophermapTemplateName:  ".gophermap",
			RequestTimeoutDuration: time.Duration(requestTimeoutSeconds) * time.Second,
			OS:                     runtime.GOOS,
			Architecture:           runtime.GOARCH,
			NumCpus:                runtime.NumCPU(),
		}
	}
	return *_configuration
}

func getEnv(key, fallback string) string {
	if value, ok := os.LookupEnv(key); ok {
		return value
	}
	return fallback
}
