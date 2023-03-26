package config

import (
	dbconfig "github.com/Kaese72/adapter-attendant/database"
	"github.com/Kaese72/sdup-lib/logging"
)

// type HTTPConfig struct {
// 	Address string `json:"address" mapstructure:"address"`
// 	Port    int    `json:"port" mapstructure:"port"`
// }

type Config struct {
	Database dbconfig.Kubernetes `json:"database" mapstructure:"database"`
	//HTTPConfig HTTPConfig          `json:"http-server" mapstructure:"http-server"`
	Logging logging.Config `json:"logging"`
}
