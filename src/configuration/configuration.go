package configuration

import (
	"log"
	"os"
	"runtime"
	"strconv"
	"time"

	"github.com/joho/godotenv"
)

type Configuration struct {
	Title                  string
	HostName               string
	BindAddress            string // In a Docker container, the ip will be different from the host ip
	Port                   string
	GopherRoot             string
	FireWallConfigFile     string
	FileAccessConfigFile   string
	GophermapTemplateName  string
	RequestTimeoutDuration time.Duration
	RequestMaximumBytes    int
	OS                     string
	Architecture           string
	NumCpus                int
	Metadata               *Metadata
}

var _configuration *Configuration = nil

func GetConfiguration() *Configuration {
	if _configuration == nil {
		var _ = godotenv.Load(".env")
		envRequestTimeoutSeconds := getEnv("READWRITE_TIMEOUT_SECONDS", "30")
		envRequestMaximumBytes := getEnv("REQUEST_MAXIMUM_BYTES", "1024")
		requestTimeoutSeconds := 30
		requestMaximumBytes := 1024
		if val, err := strconv.Atoi(envRequestTimeoutSeconds); err != nil {
			log.Println("Invalid value for READWRITE_TIMEOUT_SECONDS, using default value: 30")
			requestTimeoutSeconds = 30
		} else {
			requestTimeoutSeconds = val
		}
		if val, err := strconv.Atoi(envRequestMaximumBytes); err != nil {
			log.Println("Invalid value for REQUEST_MAXIMUM_BYTES, using default value: 1024")
			requestMaximumBytes = 1024
		} else {
			requestMaximumBytes = val
		}
		m, err := GetMetadata()
		if err != nil {
			log.Println("Cannot load metadata:", err)
		}
		_configuration = &Configuration{
			Title:                  getEnv("TITLE", "Wes C's Gopher Hole"),
			HostName:               getEnv("HOSTNAME", "localhost"),
			BindAddress:            getEnv("HOST_BIND_IP", "0.0.0.0"),
			Port:                   getEnv("PORT", "70"),
			GopherRoot:             getEnv("GOPHER_ROOT", "gopher-root"),
			FireWallConfigFile:     getEnv("FIREWALL_CONFIG_FILE", "firewall-config.json"),
			FileAccessConfigFile:   getEnv("FILE_ACCESS_CONFIG_FILE", "file-access-config.json"),
			GophermapTemplateName:  ".gophermap",
			RequestTimeoutDuration: time.Duration(requestTimeoutSeconds) * time.Second,
			RequestMaximumBytes:    requestMaximumBytes,
			OS:                     runtime.GOOS,
			Architecture:           runtime.GOARCH,
			NumCpus:                runtime.NumCPU(),
			Metadata:               m,
		}
	}
	return _configuration
}

func getEnv(key, fallback string) string {
	if value, ok := os.LookupEnv(key); ok {
		return value
	}
	return fallback
}
