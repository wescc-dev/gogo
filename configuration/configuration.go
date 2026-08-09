package configuration

import (
	"os"

	"github.com/joho/godotenv"
)

type Configuration struct {
	Title string
	Host  string
	Port  string
}

var _configuration *Configuration = nil

func GetConfiguration() Configuration {
	godotenv.Load(".env")
	if _configuration == nil {
		_configuration = &Configuration{
			Title: os.Getenv("TITLE"),
			Host:  os.Getenv("HOST"),
			Port:  os.Getenv("PORT"),
		}
	}
	return *_configuration
}
