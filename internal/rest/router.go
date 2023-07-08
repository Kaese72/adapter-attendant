package rest

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"

	"github.com/Kaese72/adapter-attendant/internal/database"
	"github.com/Kaese72/adapter-attendant/internal/logging"
	"github.com/Kaese72/adapter-attendant/rest/models"
	"github.com/gorilla/mux"
	"go.elastic.co/apm/module/apmgorilla"
)

func ServeHTTPError(err error, writer http.ResponseWriter, ctx context.Context) {
	logging.Error(fmt.Sprintf("Serving HTTP error because of error <- '%s'", err.Error()), ctx)
	http.Error(writer, fmt.Sprintf("Default error because I am lazy <- %s", err.Error()), http.StatusNotFound)
}

func InitRest(database database.AdapterAttendantDB) *mux.Router {
	router := mux.NewRouter()
	apmgorilla.Instrument(router)

	//Everything else (not /auth/login) should have the authentication middleware
	apiv0 := router.PathPrefix("/adapter-attendant/v0/").Subrouter()

	apiv0.HandleFunc("/adapters", func(writer http.ResponseWriter, reader *http.Request) {
		ctx := reader.Context()
		adapterNames, err := database.GetAdapters(ctx)
		if err != nil {
			ServeHTTPError(err, writer, ctx)
			return
		}
		bytes, err := json.Marshal(adapterNames)
		if err != nil {
			ServeHTTPError(err, writer, ctx)
			return
		}
		// FIXME Check amount of bytes written
		_, err = writer.Write(bytes)
		if err != nil {
			logging.Error(fmt.Sprintf("Failed to write response <- '%s'", err.Error()), ctx)
		}
	}).Methods("GET")

	apiv0.HandleFunc("/adapters/{name}", func(writer http.ResponseWriter, reader *http.Request) {
		adapter := models.Adapter{}
		ctx := reader.Context()
		err := json.NewDecoder(reader.Body).Decode(&adapter)
		if err != nil {
			ServeHTTPError(err, writer, ctx)
			return
		}
		// Override adapter.Name based on REST endpoint
		vars := mux.Vars(reader)
		deviceID := vars["name"]
		adapter.Name = deviceID

		if adapter.Image == nil {
			if err != nil {
				ServeHTTPError(errors.New("must set image information"), writer, ctx)
				return
			}
		}
		if adapter.Config == nil {
			if err != nil {
				ServeHTTPError(errors.New("must set config information"), writer, ctx)
				return
			}
		}

		dbAdapter, err := database.ApplyAdapter(adapter.Intermediary(), ctx)
		if err != nil {
			ServeHTTPError(err, writer, ctx)
			return
		}
		adapter = models.AdapterFromIntermediary(dbAdapter)

		// Start
		bytes, err := json.Marshal(adapter)
		if err != nil {
			ServeHTTPError(err, writer, ctx)
			return
		}
		// FIXME Check amount of bytes written
		_, err = writer.Write(bytes)
		if err != nil {
			logging.Error(fmt.Sprintf("Failed to write response <- '%s'", err.Error()), ctx)
		}

	}).Methods("PUT")

	apiv0.HandleFunc("/adapters/{name}", func(writer http.ResponseWriter, reader *http.Request) {
		adapter := models.Adapter{}
		ctx := reader.Context()
		err := json.NewDecoder(reader.Body).Decode(&adapter)
		if err != nil {
			ServeHTTPError(err, writer, ctx)
			return
		}
		// Override adapter.Name based on REST endpoint
		vars := mux.Vars(reader)
		deviceID := vars["name"]
		adapter.Name = deviceID

		dbAdapter, err := database.ApplyAdapter(adapter.Intermediary(), ctx)
		if err != nil {
			ServeHTTPError(err, writer, ctx)
			return
		}
		adapter = models.AdapterFromIntermediary(dbAdapter)

		// Start
		bytes, err := json.Marshal(adapter)
		if err != nil {
			ServeHTTPError(err, writer, ctx)
			return
		}
		// FIXME Check amount of bytes written
		_, err = writer.Write(bytes)
		if err != nil {
			logging.Error(fmt.Sprintf("Failed to write response <- '%s'", err.Error()), ctx)
		}

	}).Methods("PATCH")

	apiv0.HandleFunc("/adapters/{name}", func(writer http.ResponseWriter, reader *http.Request) {
		vars := mux.Vars(reader)
		ctx := reader.Context()
		adapterName := vars["name"]

		dbAdapter, err := database.GetAdapter(adapterName, ctx)
		if err != nil {
			ServeHTTPError(err, writer, ctx)
			return
		}
		adapter := models.AdapterFromIntermediary(dbAdapter)

		bytes, err := json.Marshal(adapter)
		if err != nil {
			ServeHTTPError(err, writer, ctx)
			return
		}
		// FIXME Check amount of bytes written
		_, err = writer.Write(bytes)
		if err != nil {
			logging.Error(fmt.Sprintf("Failed to write response <- '%s'", err.Error()), ctx)
		}

	}).Methods("GET")

	return router
}
