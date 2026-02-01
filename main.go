package main

import (
	"context"
	"fmt"
	"net/http"
	"os"

	"github.com/Kaese72/adapter-attendant/internal/config"
	"github.com/Kaese72/adapter-attendant/internal/database"
	"github.com/Kaese72/adapter-attendant/internal/logging"
	"github.com/Kaese72/adapter-attendant/internal/restwebapp"
	"github.com/danielgtaylor/huma/v2"
	"github.com/danielgtaylor/huma/v2/adapters/humamux"
	"github.com/gorilla/mux"
	"go.elastic.co/apm/module/apmsql"
	_ "go.elastic.co/apm/module/apmsql/mysql"
)

func init() {

}

func main() {
	kubernetesHandle, err := database.NewPureK8sBackend(config.Loaded.ClusterConfig)
	if err != nil {
		logging.Error(err.Error(), context.Background())
		os.Exit(1)
	}

	db, err := apmsql.Open("mysql", fmt.Sprintf("%s:%s@tcp(%s:%d)/%s?parseTime=true&loc=UTC", config.Loaded.Database.User, config.Loaded.Database.Password, config.Loaded.Database.Host, config.Loaded.Database.Port, config.Loaded.Database.Database))
	if err != nil {
		logging.Error(err.Error(), context.Background())
		os.Exit(1)
	}
	restWebapp := restwebapp.NewWebApp(kubernetesHandle, db)

	// Create Huma API
	router := mux.NewRouter()
	humaConfig := huma.DefaultConfig("adapter-attendant", "1.0.0")
	humaConfig.OpenAPIPath = "/adapter-attendant/openapi"
	humaConfig.DocsPath = "/adapter-attendant/docs"
	api := humamux.New(router, humaConfig)

	// Adapter Attendant endpoints
	// V1 API
	huma.Get(api, "/adapter-attendant/v1/adapters", restWebapp.GetAdaptersV1)
	huma.Post(api, "/adapter-attendant/v1/adapters", restWebapp.PostAdapterV1)
	huma.Get(api, "/adapter-attendant/v1/adapters/{id}", restWebapp.GetAdapterV1)
	huma.Delete(api, "/adapter-attendant/v1/adapters/{id}", restWebapp.DeleteAdapterV1)
	huma.Post(api, "/adapter-attendant/v1/adapters/{id}/sync", restWebapp.SyncAdapterV1)
	huma.Post(api, "/adapter-attendant/v1/adapters/{id}/update", restWebapp.UpdateAdapterV1)
	huma.Get(api, "/adapter-attendant/v1/adapters/{id}/address", restWebapp.GetAdapterAddressV1)
	huma.Get(api, "/adapter-attendant/v1/adapters/{id}/arguments", restWebapp.GetAdapterArgumentsForAdapterV1)
	huma.Post(api, "/adapter-attendant/v1/adapters/{id}/arguments", restWebapp.PostAdapterArgumentsForAdapterV1)
	huma.Delete(api, "/adapter-attendant/v1/adapters/{id}/arguments/{argumentId}", restWebapp.DeleteAdapterArgumentsForAdapterV1)
	huma.Patch(api, "/adapter-attendant/v1/adapters/{adapterId}/arguments/{argumentId}", restWebapp.PatchAdapterArgumentsForAdapterV1)

	// Start the server
	if err := http.ListenAndServe("0.0.0.0:8080", router); err != nil {
		logging.Error(err.Error(), context.Background())
		os.Exit(1)
	}
}
