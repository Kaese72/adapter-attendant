package main

import (
	"context"
	"net/http"
	"os"

	"github.com/Kaese72/adapter-attendant/internal/config"
	"github.com/Kaese72/adapter-attendant/internal/database"
	"github.com/Kaese72/adapter-attendant/internal/logging"
	"github.com/Kaese72/adapter-attendant/internal/restwebapp"
	"github.com/danielgtaylor/huma/v2"
	"github.com/danielgtaylor/huma/v2/adapters/humamux"
	"github.com/gorilla/mux"
)

func init() {

}

func main() {
	dbHandle, err := database.NewAdapterAttendantDB(config.Loaded.Database)
	if err != nil {
		logging.Error(err.Error(), context.Background())
		os.Exit(1)
	}

	restWebapp := restwebapp.NewWebApp(dbHandle)

	// Create Huma API
	router := mux.NewRouter()
	humaConfig := huma.DefaultConfig("adapter-attendant", "1.0.0")
	humaConfig.OpenAPIPath = "/adapter-attendant/openapi"
	humaConfig.DocsPath = "/adapter-attendant/docs"
	api := humamux.New(router, humaConfig)

	// Adapter Attendant endpoints
	huma.Get(api, "/adapter-attendant/v0/adapters", restWebapp.GetAdapters)
	huma.Get(api, "/adapter-attendant/v0/adapters/{name}", restWebapp.GetAdapter)
	huma.Put(api, "/adapter-attendant/v0/adapters/{name}", restWebapp.PutAdapter)
	huma.Patch(api, "/adapter-attendant/v0/adapters/{name}", restWebapp.PatchAdapter)

	// Start the server
	if err := http.ListenAndServe("0.0.0.0:8080", router); err != nil {
		logging.Error(err.Error(), context.Background())
		os.Exit(1)
	}
}
