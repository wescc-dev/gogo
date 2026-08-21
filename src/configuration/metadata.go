package configuration

import (
	"embed"
	"encoding/json"
)

//go:embed _embed/metadata.json
var metadataFs embed.FS

var metadata *Metadata

// BuildVersion is set by the build process.
//
//	 go build \
//			-ldflags="-X gogo/src/configuration.BuildVersion=${VERSION} -extldflags=-static" \
//			-o gogo ./src
var BuildVersion string = ""

type Metadata struct {
	AppName   string `json:"app_name"`
	Version   string `json:"version"`
	Copyright string `json:"copyright"`
	License   string `json:"license"`
	Link      string `json:"link"`
	Footer    string `json:"footer"`
}

func init() {
	m, err := loadMetadata()
	if err != nil {
		return
	}
	metadata = m
}

func GetMetadata() (*Metadata, error) {
	if metadata == nil {
		m, err := loadMetadata()
		if err != nil {
			return nil, err
		}
		metadata = m
	}
	return metadata, nil
}

func loadMetadata() (*Metadata, error) {
	data, err := metadataFs.ReadFile("_embed/metadata.json")
	if err != nil {
		return &Metadata{}, err
	}
	var meta Metadata
	err = json.Unmarshal(data, &meta)
	if err != nil {
		return &Metadata{}, err
	}
	if BuildVersion != "" {
		meta.Version = BuildVersion
	}

	return &meta, err
}
