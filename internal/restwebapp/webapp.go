package restwebapp

import (
	"context"

	"github.com/Kaese72/adapter-attendant/internal/database"
	"github.com/Kaese72/adapter-attendant/rest/models"
	"github.com/danielgtaylor/huma/v2"
)

type webApp struct {
	database database.AdapterAttendantDB
}

func NewWebApp(database database.AdapterAttendantDB) webApp {
	return webApp{
		database: database,
	}
}

// GetAdapters returns all adapters
func (app webApp) GetAdapters(ctx context.Context, input *struct{}) (*struct {
	Body []string
}, error) {
	adapterNames, err := app.database.GetAdapters(ctx)
	if err != nil {
		return nil, err
	}
	return &struct {
		Body []string
	}{
		Body: adapterNames,
	}, nil
}

// GetAdapter returns a specific adapter by name
func (app webApp) GetAdapter(ctx context.Context, input *struct {
	Name string `path:"name" doc:"the name of the adapter to retrieve"`
}) (*struct {
	Body models.Adapter
}, error) {
	dbAdapter, err := app.database.GetAdapter(input.Name, ctx)
	if err != nil {
		return nil, huma.Error404NotFound("adapter not found")
	}
	adapter := models.AdapterFromIntermediary(dbAdapter)
	return &struct {
		Body models.Adapter
	}{
		Body: adapter,
	}, nil
}

// PutAdapter creates or updates an adapter
func (app webApp) PutAdapter(ctx context.Context, input *struct {
	Name string         `path:"name" doc:"the name of the adapter"`
	Body models.Adapter `body:""`
}) (*struct {
	Body models.Adapter
}, error) {
	// Override adapter.Name based on REST endpoint
	adapter := input.Body
	adapter.Name = input.Name

	if adapter.Image == nil {
		return nil, huma.Error400BadRequest("must set image information")
	}
	if adapter.Config == nil {
		return nil, huma.Error400BadRequest("must set config information")
	}

	dbAdapter, err := app.database.ApplyAdapter(adapter.Intermediary(), ctx)
	if err != nil {
		return nil, err
	}
	resultAdapter := models.AdapterFromIntermediary(dbAdapter)

	return &struct {
		Body models.Adapter
	}{
		Body: resultAdapter,
	}, nil
}

// PatchAdapter partially updates an adapter
func (app webApp) PatchAdapter(ctx context.Context, input *struct {
	Name string         `path:"name" doc:"the name of the adapter"`
	Body models.Adapter `body:""`
}) (*struct {
	Body models.Adapter
}, error) {
	// Override adapter.Name based on REST endpoint
	adapter := input.Body
	adapter.Name = input.Name

	dbAdapter, err := app.database.ApplyAdapter(adapter.Intermediary(), ctx)
	if err != nil {
		return nil, err
	}
	resultAdapter := models.AdapterFromIntermediary(dbAdapter)

	return &struct {
		Body models.Adapter
	}{
		Body: resultAdapter,
	}, nil
}
