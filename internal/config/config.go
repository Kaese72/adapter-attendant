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

type Kubernetes struct {
	KubeConfigPath string `json:"kubeconfig-path" mapstructure:"kubeconfig-path"`
	InCluster      bool   `json:"in-cluster" mapstructure:"in-cluster"`
	NameSpace      string `json:"namespace" mapstructure:"namespace"`
	// ApiserverProxyURL, if set, routes in-cluster apiserver calls through an
	// HTTP CONNECT proxy dedicated to reaching the apiserver (e.g.
	// kube-apiserver-proxy) instead of the ClusterIP directly -- see
	// database.NewPureK8sBackend. Deliberately its own setting rather than a
	// generic HTTPS_PROXY env var: this process has no other outbound HTTPS
	// calls today, but a blanket HTTPS_PROXY would silently capture any added
	// later too, and the apiserver-only proxy refuses to CONNECT anywhere
	// else.
	ApiserverProxyURL string `json:"apiserver-proxy-url" mapstructure:"apiserver-proxy-url"`
}

type Adapters struct {
	DeviceStoreURL       string `json:"device-store-url" mapstructure:"device-store-url"`
	DeviceStoreJWTSecret string `json:"device-store-jwt-secret" mapstructure:"device-store-jwt-secret"`
}

type Auth struct {
	RSAPublicKeyPath string `json:"rsa-public-key-path" mapstructure:"rsa-public-key-path"`
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
	// Kubernetes access configuration
	viper.BindEnv("kubernetes.kubeconfig-path")
	viper.BindEnv("kubernetes.adapter-namespace")
	// kubernetes.in-cluster is treated as a fallback when no config is provided and
	// thus defaults to true
	viper.BindEnv("kubernetes.in-cluster")
	viper.SetDefault("kubernetes.in-cluster", true)
	viper.BindEnv("kubernetes.apiserver-proxy-url")

	// # Logging
	viper.BindEnv("logging.stdout")
	viper.SetDefault("logging.stdout", true)
	viper.BindEnv("logging.http.url")

	// # Device Store
	viper.BindEnv("adapters.device-store-url")
	viper.BindEnv("adapters.device-store-jwt-secret")

	// # Authentication service public key (RS256 use-token verification)
	viper.BindEnv("auth.rsa-public-key-path")

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
			KubeConfigPath:    viper.GetString("kubernetes.kubeconfig-path"),
			NameSpace:         viper.GetString("kubernetes.adapter-namespace"),
			InCluster:         viper.GetBool("kubernetes.in-cluster"),
			ApiserverProxyURL: viper.GetString("kubernetes.apiserver-proxy-url"),
		},
		Adapters: Adapters{
			DeviceStoreURL:       viper.GetString("adapters.device-store-url"),
			DeviceStoreJWTSecret: viper.GetString("adapters.device-store-jwt-secret"),
		},
		Auth: Auth{
			RSAPublicKeyPath: viper.GetString("auth.rsa-public-key-path"),
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
