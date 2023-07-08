package database

import (
	"context"

	"github.com/Kaese72/adapter-attendant/internal/config"
	"github.com/Kaese72/adapter-attendant/internal/database/intermediaries"
)

type AdapterAttendantDB interface {
	GetAdapter(string, context.Context) (intermediaries.Adapter, error)
	GetAdapters(context.Context) ([]string, error)
	ApplyAdapter(intermediaries.Adapter, context.Context) (intermediaries.Adapter, error)
}

func NewAdapterAttendantDB(dbconf config.Kubernetes) (AdapterAttendantDB, error) {
	// FIXME Do we want any other kind?
	return NewPureK8sBackend(dbconf)
}
