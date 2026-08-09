package configuration

import (
	"os"

	"github.com/joho/godotenv"
)

type Configuration struct {
	Title      string
	Host       string
	HostBindIp string // In a Docker container, the ip will be different from the host ip
	Port       string
	GopherRoot string
}

var _configuration *Configuration = nil

func GetConfiguration() Configuration {
	var _ = godotenv.Load(".env")
	if _configuration == nil {
		_configuration = &Configuration{
			Title:      getEnv("TITLE", "Wes C's Gopher Hole"),
			Host:       getEnv("HOST", "localhost"),
			HostBindIp: getEnv("HOST_BIND_IP", "0.0.0.0"),
			Port:       getEnv("PORT", "70"),
			GopherRoot: getEnv("GOPHER_ROOT", "./gopher-root"),
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
