package configuration

import (
	"os"
	"strconv"
	"time"

	"github.com/joho/godotenv"
)

const (
	AppName   = "Wes C's Go Gopher Server"
	Version   = "0.0.1"
	Copyright = "Copyright©️ 2026 Wes C"
	Link      = "https://wesc.neocities.org/#/gopherhole"
)

type Configuration struct {
	Title              string
	Host               string
	BindAddress        string // In a Docker container, the ip will be different from the host ip
	Port               string
	GopherRoot         string
	FireWallConfigFile string
	IdleTimeout        time.Duration
	ReadWriteTimeout   time.Duration
}

var _configuration *Configuration = nil

func GetConfiguration() Configuration {
	var _ = godotenv.Load(".env")
	if _configuration == nil {
		var envIdleTimeout = getEnv("IDLE_TIMEOUT_SECONDS", "10")
		var envReadWriteTimeout = getEnv("READWRITE_TIMEOUT_SECONDS", "30")
		var idle int
		var readWriteTimeout int
		if val, err := strconv.Atoi(envIdleTimeout); err != nil {
			idle = 10
		} else {
			idle = val
		}
		if val, err := strconv.Atoi(envReadWriteTimeout); err != nil {
			readWriteTimeout = 30
		} else {
			readWriteTimeout = val
		}
		_configuration = &Configuration{
			Title:              getEnv("TITLE", "Wes C's Gopher Hole"),
			Host:               getEnv("HOST", "localhost"),
			BindAddress:        getEnv("HOST_BIND_IP", "0.0.0.0"),
			Port:               getEnv("PORT", "70"),
			GopherRoot:         getEnv("GOPHER_ROOT", "./gopher-root"),
			FireWallConfigFile: getEnv("FIREWALL_CONFIG_FILE", "./firewall-config.json"),
			IdleTimeout:        time.Duration(idle) * time.Second,
			ReadWriteTimeout:   time.Duration(readWriteTimeout) * time.Second,
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
