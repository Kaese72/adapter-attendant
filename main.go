package main

import (
	"net/http"

	"github.com/Kaese72/adapter-attendant/internal/config"
	"github.com/Kaese72/adapter-attendant/internal/database"
	"github.com/Kaese72/adapter-attendant/internal/rest"
)

func init() {

}

func main() {
	dbHandle, err := database.NewAdapterAttendantDB(config.Loaded.Database)
	if err != nil {
		panic(err.Error())
	}

	httpMux := rest.InitRest(dbHandle)
	http.ListenAndServe("0.0.0.0:8080", httpMux)
}
