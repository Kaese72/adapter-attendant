package config

import (
	"strings"

	"github.com/spf13/viper"
)

type Database struct {
	Host     string `json:"host" mapstructure:"host" `
	Port     int    `json:"port" mapstructure:"port"`
	User     string `json:"user" mapstructure:"user"`
	Password string `json:"password" mapstructure:"password"`
	Database string `json:"database" mapstructure:"database"`
}

// Kubernetes describes where ArgoCD deploys the adapters. This service never
// talks to the Kubernetes API itself; it only needs the namespace to tell
// device-store an adapter's in-cluster address.
type Kubernetes struct {
	NameSpace string `json:"namespace" mapstructure:"namespace"`
}

type Adapters struct {
	DeviceStoreURL       string `json:"device-store-url" mapstructure:"device-store-url"`
	DeviceStoreJWTSecret string `json:"device-store-jwt-secret" mapstructure:"device-store-jwt-secret"`
}

type Auth struct {
	RSAPublicKeyPath string `json:"rsa-public-key-path" mapstructure:"rsa-public-key-path"`
	// PluginTokens are the bearer tokens the ArgoCD ApplicationSet Plugin
	// generator may present, comma-separated so the token can be rotated by
	// adding the new value before removing the old one.
	PluginTokens string `json:"plugin-tokens" mapstructure:"plugin-tokens"`
	// InternalURL is the authentication service's internal listener, e.g.
	// http://authentication:8081. When set, API tokens are accepted and
	// looked up there; when empty, only use tokens are.
	InternalURL string `json:"internal-url" mapstructure:"internal-url"`
	// ServiceAccountTokenPath is this pod's projected, audience-bound
	// ServiceAccount token, presented to the authentication service when
	// looking an API token up.
	ServiceAccountTokenPath string `json:"service-account-token-path" mapstructure:"service-account-token-path"`
}

type Config struct {
	ClusterConfig Kubernetes `json:"cluster-config" mapstructure:"cluster-config"`
	Adapters      Adapters   `json:"adapters" mapstructure:"adapters"`
	Auth          Auth       `json:"auth" mapstructure:"auth"`
	Database      Database   `json:"database" mapstructure:"database"`
	PublicPort    int        `json:"public-port" mapstructure:"public-port"`
	InternalPort  int        `json:"internal-port" mapstructure:"internal-port"`
}

// Loaded contains all configuration which was loaded in when the application started
var Loaded Config

func init() {
	// We have elected to no use AutomaticEnv() because of https://github.com/spf13/viper/issues/584
	// myVip.AutomaticEnv()
	// Set replaces to allow keys like "database.mongodb.connection-string"
	viper.SetEnvKeyReplacer(strings.NewReplacer(".", "_", "-", "_"))

	// # Default values where appropriate
	// Namespace the adapters are deployed into
	viper.BindEnv("kubernetes.adapter-namespace")

	// # Logging
	viper.BindEnv("logging.stdout")
	viper.SetDefault("logging.stdout", true)
	viper.BindEnv("logging.http.url")

	// # Device Store
	viper.BindEnv("adapters.device-store-url")
	viper.BindEnv("adapters.device-store-jwt-secret")

	// # Authentication service public key (RS256 use-token verification)
	viper.BindEnv("auth.rsa-public-key-path")
	// # ArgoCD ApplicationSet Plugin generator tokens
	viper.BindEnv("auth.plugin-tokens")
	// API token introspection (optional; see AuthConfig.InternalURL)
	viper.BindEnv("auth.internal-url")
	viper.BindEnv("auth.service-account-token-path")
	viper.SetDefault("auth.service-account-token-path", "/var/run/secrets/tokens/authentication-internal")

	// # Ports
	viper.BindEnv("public-port")
	viper.SetDefault("public-port", 8080)
	viper.BindEnv("internal-port")
	viper.SetDefault("internal-port", 8081)

	// # Database configuration, if left out.
	viper.BindEnv("database.host")
	viper.BindEnv("database.port")
	viper.BindEnv("database.user")
	viper.BindEnv("database.password")
	viper.BindEnv("database.database")
	viper.SetDefault("database.port", 3306)
	viper.SetDefault("database.database", "adapterattendant")

	// FIXME check that the required config options are set

	Loaded = Config{
		ClusterConfig: Kubernetes{
			NameSpace: viper.GetString("kubernetes.adapter-namespace"),
		},
		Adapters: Adapters{
			DeviceStoreURL:       viper.GetString("adapters.device-store-url"),
			DeviceStoreJWTSecret: viper.GetString("adapters.device-store-jwt-secret"),
		},
		Auth: Auth{
			RSAPublicKeyPath:        viper.GetString("auth.rsa-public-key-path"),
			PluginTokens:            viper.GetString("auth.plugin-tokens"),
			InternalURL:             viper.GetString("auth.internal-url"),
			ServiceAccountTokenPath: viper.GetString("auth.service-account-token-path"),
		},
		Database: Database{
			Host:     viper.GetString("database.host"),
			Port:     viper.GetInt("database.port"),
			User:     viper.GetString("database.user"),
			Password: viper.GetString("database.password"),
			Database: viper.GetString("database.database"),
		},
		PublicPort:   viper.GetInt("public-port"),
		InternalPort: viper.GetInt("internal-port"),
	}
}
