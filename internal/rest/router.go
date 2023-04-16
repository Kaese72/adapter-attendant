package rest

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"

	"github.com/Kaese72/adapter-attendant/internal/database"
	"github.com/Kaese72/adapter-attendant/rest/models"
	"github.com/Kaese72/huemie-lib/logging"
	"github.com/gorilla/mux"
)

func ServeHTTPError(err error, writer http.ResponseWriter) {
	logging.Error(fmt.Sprintf("Serving HTTP error because of error <- '%s'", err.Error()))
	http.Error(writer, fmt.Sprintf("Default error because I am lazy <- %s", err.Error()), http.StatusNotFound)
}

func InitRest(database database.AdapterAttendantDB) *mux.Router {
	router := mux.NewRouter()

	//Everything else (not /auth/login) should have the authentication middleware
	apiv0 := router.PathPrefix("/rest/v0/").Subrouter()

	apiv0.HandleFunc("/adapters", func(writer http.ResponseWriter, reader *http.Request) {
		adapterNames, err := database.GetAdapters()
		if err != nil {
			ServeHTTPError(err, writer)
			return
		}
		bytes, err := json.Marshal(adapterNames)
		if err != nil {
			ServeHTTPError(err, writer)
			return
		}
		// FIXME Check amount of bytes written
		_, err = writer.Write(bytes)
		if err != nil {
			logging.Error(fmt.Sprintf("Failed to write response <- '%s'", err.Error()))
		}
	}).Methods("GET")

	apiv0.HandleFunc("/adapters/{name}", func(writer http.ResponseWriter, reader *http.Request) {
		adapter := models.Adapter{}
		err := json.NewDecoder(reader.Body).Decode(&adapter)
		if err != nil {
			ServeHTTPError(err, writer)
			return
		}
		// Override adapter.Name based on REST endpoint
		vars := mux.Vars(reader)
		deviceID := vars["name"]
		adapter.Name = deviceID

		if adapter.Image == nil {
			if err != nil {
				ServeHTTPError(errors.New("must set image information"), writer)
				return
			}
		}
		if adapter.Config == nil {
			if err != nil {
				ServeHTTPError(errors.New("must set config information"), writer)
				return
			}
		}

		dbAdapter, err := database.ApplyAdapter(adapter.Intermediary())
		if err != nil {
			ServeHTTPError(err, writer)
			return
		}
		adapter = models.AdapterFromIntermediary(dbAdapter)

		// Start
		bytes, err := json.Marshal(adapter)
		if err != nil {
			ServeHTTPError(err, writer)
			return
		}
		// FIXME Check amount of bytes written
		_, err = writer.Write(bytes)
		if err != nil {
			logging.Error(fmt.Sprintf("Failed to write response <- '%s'", err.Error()))
		}

	}).Methods("PUT")

	apiv0.HandleFunc("/adapters/{name}", func(writer http.ResponseWriter, reader *http.Request) {
		adapter := models.Adapter{}
		err := json.NewDecoder(reader.Body).Decode(&adapter)
		if err != nil {
			ServeHTTPError(err, writer)
			return
		}
		// Override adapter.Name based on REST endpoint
		vars := mux.Vars(reader)
		deviceID := vars["name"]
		adapter.Name = deviceID

		dbAdapter, err := database.ApplyAdapter(adapter.Intermediary())
		if err != nil {
			ServeHTTPError(err, writer)
			return
		}
		adapter = models.AdapterFromIntermediary(dbAdapter)

		// Start
		bytes, err := json.Marshal(adapter)
		if err != nil {
			ServeHTTPError(err, writer)
			return
		}
		// FIXME Check amount of bytes written
		_, err = writer.Write(bytes)
		if err != nil {
			logging.Error(fmt.Sprintf("Failed to write response <- '%s'", err.Error()))
		}

	}).Methods("PATCH")

	apiv0.HandleFunc("/adapters/{name}", func(writer http.ResponseWriter, reader *http.Request) {
		vars := mux.Vars(reader)
		adapterName := vars["name"]

		dbAdapter, err := database.GetAdapter(adapterName)
		if err != nil {
			ServeHTTPError(err, writer)
			return
		}
		adapter := models.AdapterFromIntermediary(dbAdapter)

		bytes, err := json.Marshal(adapter)
		if err != nil {
			ServeHTTPError(err, writer)
			return
		}
		// FIXME Check amount of bytes written
		_, err = writer.Write(bytes)
		if err != nil {
			logging.Error(fmt.Sprintf("Failed to write response <- '%s'", err.Error()))
		}

	}).Methods("GET")

	return router
}
