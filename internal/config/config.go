package config

import (
	"strings"

	"github.com/spf13/viper"
)

type Kubernetes struct {
	KubeConfigPath string `json:"kubeconfig-path" mapstructure:"kubeconfig-path"`
	InCluster      bool   `json:"in-cluster" mapstructure:"in-cluster"`
	NameSpace      string `json:"namespace" mapstructure:"namespace"`
}

type Config struct {
	Database       Kubernetes `json:"database" mapstructure:"database"`
	DeviceStoreURL string     `json:"device-store-url" mapstructure:"device-store-url"`
}

// Loaded contains all configuration which was loaded in when the application started
var Loaded Config

func init() {
	// We have elected to no use AutomaticEnv() because of https://github.com/spf13/viper/issues/584
	// myVip.AutomaticEnv()
	// Set replaces to allow keys like "database.mongodb.connection-string"
	viper.SetEnvKeyReplacer(strings.NewReplacer(".", "_", "-", "_"))

	// # Default values where appropriate
	// Kubernetes access configuration
	viper.BindEnv("kubernetes.kubeconfig-path")
	viper.BindEnv("kubernetes.adapter-namespace")
	viper.SetDefault("kubernetes.adapter-namespace", "adapters")
	// kubernetes.in-cluster is treated as a fallback when no config is provided and
	// thus defaults to true
	viper.BindEnv("kubernetes.in-cluster")
	viper.SetDefault("kubernetes.in-cluster", true)

	// # Logging
	viper.BindEnv("logging.stdout")
	viper.SetDefault("logging.stdout", true)
	viper.BindEnv("logging.http.url")

	// # Device Store
	viper.BindEnv("device-store.url")
	viper.SetDefault("device-store.url", "http://device-store.default:8080")

	Loaded = Config{
		Database: Kubernetes{
			KubeConfigPath: viper.GetString("kubernetes.kubeconfig-path"),
			NameSpace:      viper.GetString("kubernetes.adapter-namespace"),
			InCluster:      viper.GetBool("kubernetes.in-cluster"),
		},
		DeviceStoreURL: viper.GetString("device-store.url"),
	}
}
