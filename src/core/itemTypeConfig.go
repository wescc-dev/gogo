package core

import (
	"encoding/json"
	"os"
	"slices"

	"github.com/wescc-dev/gogo/src/configuration"
)

type typeMappings struct {
	ItemType   string   `json:"itemType"`
	Pictogram  string   `json:"pictogram"`
	Extensions []string `json:"extensions"`
}

type defaultItemType struct {
	ItemType  string `json:"itemType"`
	Pictogram string `json:"pictogram"`
}

type ItemTypeConfig struct {
	PictogramsEnabled bool            `json:"pictogramsEnabled"`
	DefaultItemType   defaultItemType `json:"defaultItemType"`
	ItemsTypes        []typeMappings  `json:"itemsTypes"`
}

var cfg = configuration.GetConfiguration()

var itemTypeConfig *ItemTypeConfig

func init() {
	_ = loadConfig()
}

func loadConfig() error {
	if itemTypeConfig != nil {
		return nil
	}
	data, err := os.ReadFile(cfg.ItemTypeConfigFile)
	if err != nil {
		return err
	}
	itemTypeConfig = &ItemTypeConfig{}
	if err := json.Unmarshal(data, itemTypeConfig); err != nil {
		return err
	}
	return nil
}

func GetItemTypeByExtension(ext string) (string, string) {
	for _, itemType := range itemTypeConfig.ItemsTypes {
		if slices.Contains(itemType.Extensions, ext) {
			if itemTypeConfig.PictogramsEnabled {
				return itemType.ItemType, itemType.Pictogram
			}
			return itemType.ItemType, ""
		}
	}
	if itemTypeConfig.PictogramsEnabled {
		return itemTypeConfig.DefaultItemType.ItemType, itemTypeConfig.DefaultItemType.Pictogram
	}
	return itemTypeConfig.DefaultItemType.ItemType, ""
}
