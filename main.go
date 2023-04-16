package main

import (
	"net/http"
	"strings"

	"github.com/Kaese72/adapter-attendant/internal/config"
	"github.com/Kaese72/adapter-attendant/internal/database"
	"github.com/Kaese72/adapter-attendant/internal/rest"
	"github.com/Kaese72/huemie-lib/logging"
	"github.com/spf13/viper"
)

func main() {
	// conf, err := rest.InClusterConfig()
	// # Viper configuration
	myVip := viper.New()
	// We have elected to no use AutomaticEnv() because of https://github.com/spf13/viper/issues/584
	// myVip.AutomaticEnv()
	// Set replaces to allow keys like "database.mongodb.connection-string"
	myVip.SetEnvKeyReplacer(strings.NewReplacer(".", "_", "-", "_"))

	// # Configuration file configuration
	myVip.SetConfigName("config")
	myVip.AddConfigPath(".")
	myVip.AddConfigPath("99_local")
	myVip.AddConfigPath("/etc/adapter-attendant/")
	if err := myVip.ReadInConfig(); err != nil {
		logging.Error(err.Error())
	}

	// # Default values where appropriate
	// Kubernetes access configuration
	myVip.BindEnv("kubernetes.kubeconfig-path")
	myVip.BindEnv("kubernetes.adapter-namespace")
	myVip.SetDefault("kubernetes.adapter-namespace", "adapters")

	// # Logging
	myVip.BindEnv("logging.stdout")
	myVip.SetDefault("logging.stdout", true)
	myVip.BindEnv("logging.http.url")

	conf := config.Config{
		Database: database.Kubernetes{
			KubeConfigPath: myVip.GetString("kubernetes.kubeconfig-path"),
			NameSpace:      myVip.GetString("kubernetes.adapter-namespace"),
		},
	}
	dbHandle, err := database.NewAdapterAttendantDB(conf.Database)
	if err != nil {
		panic(err.Error())
	}

	httpMux := rest.InitRest(dbHandle)
	http.ListenAndServe("0.0.0.0:8080", httpMux)
}
