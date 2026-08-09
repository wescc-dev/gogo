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
}

var _configuration *Configuration = nil

func GetConfiguration() Configuration {
	godotenv.Load(".env")
	if _configuration == nil {
		_configuration = &Configuration{
			Title:      os.Getenv("TITLE"),
			Host:       os.Getenv("HOST"),
			HostBindIp: os.Getenv("HOST_BIND_IP"),
			Port:       os.Getenv("PORT"),
		}
	}
	return *_configuration
}
