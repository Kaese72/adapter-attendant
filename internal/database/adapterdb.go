package database

import "github.com/Kaese72/adapter-attendant/internal/database/intermediaries"

type AdapterAttendantDB interface {
	GetAdapter(string) (intermediaries.Adapter, error)
	GetAdapters() ([]string, error)
	ApplyAdapter(intermediaries.Adapter) (intermediaries.Adapter, error)
}

func NewAdapterAttendantDB(dbconf Kubernetes) (AdapterAttendantDB, error) {
	// FIXME Do we want any other kind?
	return NewPureK8sBackend(dbconf)
}
