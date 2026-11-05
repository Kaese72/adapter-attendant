package main

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"strings"

	"github.com/Kaese72/adapter-attendant/internal/config"
	"github.com/Kaese72/adapter-attendant/internal/logging"
	"github.com/Kaese72/adapter-attendant/internal/restwebapp"
	"github.com/Kaese72/authentication/usertoken"
	"github.com/danielgtaylor/huma/v2"
	"github.com/danielgtaylor/huma/v2/adapters/humamux"
	"github.com/gorilla/mux"
	"go.elastic.co/apm/module/apmsql"
	_ "go.elastic.co/apm/module/apmsql/mysql"
)

func parseTokenList(raw string) []string {
	var out []string
	for _, t := range strings.Split(raw, ",") {
		if t = strings.TrimSpace(t); t != "" {
			out = append(out, t)
		}
	}
	return out
}

func main() {
	pluginTokens := parseTokenList(config.Loaded.Auth.PluginTokens)
	if len(pluginTokens) == 0 {
		logging.Error("must supply auth plugin-tokens", context.Background())
		os.Exit(1)
	}

	db, err := apmsql.Open("mysql", fmt.Sprintf("%s:%s@tcp(%s:%d)/%s?parseTime=true&loc=UTC", config.Loaded.Database.User, config.Loaded.Database.Password, config.Loaded.Database.Host, config.Loaded.Database.Port, config.Loaded.Database.Database))
	if err != nil {
		logging.Error(err.Error(), context.Background())
		os.Exit(1)
	}
	restWebapp := restwebapp.NewWebApp(db, pluginTokens)

	pubKey, err := usertoken.LoadPublicKeyFromFile(config.Loaded.Auth.RSAPublicKeyPath)
	if err != nil {
		logging.Error(err.Error(), context.Background())
		os.Exit(1)
	}

	// Public router (adapter-attendant)
	publicRouter := mux.NewRouter()
	// API tokens are accepted (looked up in the authentication service) only
	// when its internal URL is configured; use tokens always are.
	authOptions := []usertoken.MiddlewareOption{usertoken.SkipPrefixes("/adapter-attendant/openapi", "/adapter-attendant/docs")}
	if config.Loaded.Auth.InternalURL != "" {
		authOptions = append(authOptions, usertoken.AcceptAPITokens(usertoken.NewIntrospector(config.Loaded.Auth.InternalURL, config.Loaded.Auth.ServiceAccountTokenPath)))
	}
	publicRouter.Use(usertoken.NewMiddleware(pubKey, authOptions...))
	publicHumaConfig := huma.DefaultConfig("adapter-attendant", "1.0.0")
	publicHumaConfig.OpenAPIPath = "/adapter-attendant/openapi"
	publicHumaConfig.DocsPath = "/adapter-attendant/docs"
	publicAPI := humamux.New(publicRouter, publicHumaConfig)

	huma.Get(publicAPI, "/adapter-attendant/v1/adapters", restWebapp.GetAdaptersV1)
	huma.Post(publicAPI, "/adapter-attendant/v1/adapters", restWebapp.PostAdapterV1)
	huma.Get(publicAPI, "/adapter-attendant/v1/adapters/{id}", restWebapp.GetAdapterV1)
	huma.Delete(publicAPI, "/adapter-attendant/v1/adapters/{id}", restWebapp.DeleteAdapterV1)
	huma.Post(publicAPI, "/adapter-attendant/v1/adapters/{id}/sync", restWebapp.SyncAdapterV1)
	huma.Post(publicAPI, "/adapter-attendant/v1/adapters/{id}/update", restWebapp.UpdateAdapterV1)
	huma.Get(publicAPI, "/adapter-attendant/v1/adapters/{id}/address", restWebapp.GetAdapterAddressV1)
	huma.Get(publicAPI, "/adapter-attendant/v1/adapters/{id}/arguments", restWebapp.GetAdapterArgumentsForAdapterV1)
	huma.Post(publicAPI, "/adapter-attendant/v1/adapters/{id}/arguments", restWebapp.PostAdapterArgumentsForAdapterV1)
	huma.Delete(publicAPI, "/adapter-attendant/v1/adapters/{id}/arguments/{argumentId}", restWebapp.DeleteAdapterArgumentsForAdapterV1)
	huma.Patch(publicAPI, "/adapter-attendant/v1/adapters/{adapterId}/arguments/{argumentId}", restWebapp.PatchAdapterArgumentsForAdapterV1)

	// Internal router (adapter-attendant-internal) — no auth, restrict via NetworkPolicy.
	// The exception is the ArgoCD Plugin generator endpoint, which also
	// checks its own plugin token.
	internalRouter := mux.NewRouter()
	internalAPI := humamux.New(internalRouter, huma.DefaultConfig("adapter-attendant-internal", "1.0.0"))

	huma.Get(internalAPI, "/adapter-attendant-internal/v1/adapters/{id}/address", restWebapp.GetAdapterAddressV1)
	huma.Post(internalAPI, "/adapter-attendant-internal/v1/argocd-plugin/api/v1/getparams.execute", restWebapp.PluginGetParams)

	go func() {
		if err := http.ListenAndServe(fmt.Sprintf("0.0.0.0:%d", config.Loaded.InternalPort), internalRouter); err != nil {
			logging.Error(err.Error(), context.Background())
			os.Exit(1)
		}
	}()

	if err := http.ListenAndServe(fmt.Sprintf("0.0.0.0:%d", config.Loaded.PublicPort), publicRouter); err != nil {
		logging.Error(err.Error(), context.Background())
		os.Exit(1)
	}
}
