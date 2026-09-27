package restwebapp

import (
	"context"
	"database/sql"
	"fmt"

	"github.com/Kaese72/adapter-attendant/internal/config"
	"github.com/Kaese72/adapter-attendant/internal/database"
	"github.com/Kaese72/adapter-attendant/internal/logging"
	"github.com/Kaese72/adapter-attendant/rest/models"
	"github.com/Kaese72/authentication/usertoken"
	"github.com/danielgtaylor/huma/v2"
)

// requireAdaptersView/requireAdaptersModify return a huma error unless the
// caller (as put in ctx by usertoken.Middleware) may view/modify adapters.
// Every endpoint in this file calls one of the two first; admins always pass.
func requireAdaptersView(ctx context.Context) error {
	permissions, ok := usertoken.PermissionsFromContext(ctx)
	if !ok || !permissions.HasView(usertoken.ResourceAdapters) {
		return huma.Error403Forbidden("view access to adapters is required")
	}
	return nil
}

func requireAdaptersModify(ctx context.Context) error {
	permissions, ok := usertoken.PermissionsFromContext(ctx)
	if !ok || !permissions.HasModify(usertoken.ResourceAdapters) {
		return huma.Error403Forbidden("modify access to adapters is required")
	}
	return nil
}

// requireAdaptersViewIfPresent is GetAdapterAddressV1's check: that endpoint
// is also registered on the unauthenticated internal router that device-store
// calls through (see main.go - restricted by NetworkPolicy instead of a
// token), where PermissionsFromContext reports !ok since no middleware ever
// ran. Treat that as the trusted internal caller rather than "no access", and
// only enforce the permission when a token context is actually present (i.e.
// the public router was used).
func requireAdaptersViewIfPresent(ctx context.Context) error {
	permissions, ok := usertoken.PermissionsFromContext(ctx)
	if ok && !permissions.HasView(usertoken.ResourceAdapters) {
		return huma.Error403Forbidden("view access to adapters is required")
	}
	return nil
}

type webApp struct {
	kubernetes database.KubeHandle
	db         *sql.DB
}

func NewWebApp(kubernetes database.KubeHandle, db *sql.DB) webApp {
	return webApp{
		kubernetes: kubernetes,
		db:         db,
	}
}

// GetAdaptersV1 returns adapters
func (app webApp) GetAdaptersV1(ctx context.Context, input *struct {
}) (*struct {
	Body []models.Adapter
}, error) {
	if err := requireAdaptersView(ctx); err != nil {
		return nil, err
	}
	retAdapters, err := app.getAdaptersV1(ctx, nil)
	if err != nil {
		return nil, err
	}
	return &struct {
		Body []models.Adapter
	}{
		Body: retAdapters,
	}, nil
}

// getAdaptersV1 is a helper function to get adapters, optionally by id
// Returns an API friendly error
func (app webApp) getAdaptersV1(ctx context.Context, id *int) ([]models.Adapter, error) {
	retAdapters := []models.Adapter{}
	query := "SELECT id, name, imageName, imageTag, created, updated, synced FROM adapters"
	queryArguments := []interface{}{}
	if id != nil {
		query += " WHERE id = ?"
		queryArguments = append(queryArguments, *id)
	}
	rows, err := app.db.QueryContext(ctx, query, queryArguments...)
	if err != nil {
		logging.Error("Database error when fetching adapters", ctx, map[string]interface{}{"ERROR": err.Error()})
		return nil, huma.Error500InternalServerError("Internal Server Error")
	}
	defer rows.Close()
	for rows.Next() {
		var retAdapter models.Adapter
		err := rows.Scan(&retAdapter.ID, &retAdapter.Name, &retAdapter.ImageName, &retAdapter.ImageTag, &retAdapter.Created, &retAdapter.Updated, &retAdapter.Synced)
		if err != nil {
			logging.Error("Database error when fetching adapter", ctx, map[string]interface{}{"ERROR": err.Error()})
			return nil, huma.Error500InternalServerError("Internal Server Error")
		}
		retAdapters = append(retAdapters, retAdapter)
	}
	return retAdapters, nil
}

// GetAdapterV1 returns a specific adapter by id
func (app webApp) GetAdapterV1(ctx context.Context, input *struct {
	Id int `path:"id" doc:"the Id of the adapter to retrieve"`
}) (*struct {
	Body models.Adapter
}, error) {
	if err := requireAdaptersView(ctx); err != nil {
		return nil, err
	}
	retAdapters, err := app.getAdaptersV1(ctx, &input.Id)
	if err != nil {
		return nil, err
	}
	if len(retAdapters) == 0 {
		return nil, huma.Error404NotFound("adapter not found")
	}
	return &struct {
		Body models.Adapter
	}{
		Body: retAdapters[0],
	}, nil
}

// PostAdapterV1 creates an adapter
func (app webApp) PostAdapterV1(ctx context.Context, input *struct {
	Body models.Adapter `body:""`
}) (*struct {
	Body models.Adapter
}, error) {
	if err := requireAdaptersModify(ctx); err != nil {
		return nil, err
	}
	// Override adapter.Name based on REST endpoint
	query := `INSERT IGNORE INTO adapters (name, imageName, imageTag) 
			  VALUES (?, ?, ?)
			  RETURNING id, name, imageName, imageTag, created, updated, synced`
	rows := app.db.QueryRowContext(ctx, query, input.Body.Name, input.Body.ImageName, input.Body.ImageTag)
	var resultAdapter models.Adapter
	err := rows.Scan(&resultAdapter.ID, &resultAdapter.Name, &resultAdapter.ImageName, &resultAdapter.ImageTag, &resultAdapter.Created, &resultAdapter.Updated, &resultAdapter.Synced)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, huma.Error409Conflict("adapter conflict")
		}
		logging.Error("Database error when inserting adapter", ctx, map[string]interface{}{"ERROR": err.Error()})
		return nil, huma.Error500InternalServerError("Internal Server Error")
	}

	return &struct {
		Body models.Adapter
	}{
		Body: resultAdapter,
	}, nil
}

// PostAdapterV1 creates an adapter
func (app webApp) DeleteAdapterV1(ctx context.Context, input *struct {
	Id int `path:"id" doc:"the Id of the adapter to delete"`
}) (*struct {
}, error) {
	if err := requireAdaptersModify(ctx); err != nil {
		return nil, err
	}
	// Override adapter.Name based on REST endpoint
	query := `DELETE FROM adapters WHERE id = ?`
	_, err := app.db.ExecContext(ctx, query, input.Id)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, huma.Error404NotFound("adapter not found")
		}
		logging.Error("Database error when deleting adapter", ctx, map[string]interface{}{"ERROR": err.Error()})
		return nil, huma.Error500InternalServerError("Internal Server Error")
	}
	return nil, nil
}

// SyncAdapterV1 triggers a sync for the adapter
func (app webApp) SyncAdapterV1(ctx context.Context, input *struct {
	Id int `path:"id" doc:"the Id of the adapter to sync"`
}) (*struct {
}, error) {
	if err := requireAdaptersModify(ctx); err != nil {
		return nil, err
	}
	syncAdapters, err := app.getAdaptersV1(ctx, &input.Id)
	if err != nil {
		return nil, err
	}
	if len(syncAdapters) == 0 {
		return nil, huma.Error404NotFound("adapter not found")
	}
	syncConfigurations, err := app.getAdapterArgumentsV1(ctx, input.Id)
	if err != nil {
		return nil, err
	}
	syncArguments := map[string]string{}
	for _, config := range syncConfigurations {
		syncArguments[config.ConfigKey] = config.ConfigValue
	}
	syncAdapter := syncAdapters[0]
	logging.Info("Starting sync for adapter", ctx, map[string]any{"ADAPTER_ID": syncAdapter.ID, "ADAPTER_NAME": syncAdapter.Name})
	err = app.kubernetes.ApplyAdapter(ctx, syncAdapter.ID, syncAdapter.ImageName+":"+syncAdapter.ImageTag, syncArguments)
	if err != nil {
		logging.Error("Error syncing adapter", ctx, map[string]any{"ERROR": err.Error(), "ADAPTER_ID": syncAdapter.ID, "ADAPTER_NAME": syncAdapter.Name})
		return nil, huma.Error500InternalServerError("Internal Server Error")
	}
	err = app.registerAdapterSynced(ctx, syncAdapter.ID)
	if err != nil {
		logging.Error("Error registering adapter sync", ctx, map[string]any{"ERROR": err.Error(), "ADAPTER_ID": syncAdapter.ID, "ADAPTER_NAME": syncAdapter.Name})
		return nil, huma.Error500InternalServerError("Internal Server Error")
	}
	return nil, nil
}

// UpdateAdapterV1 updates the image tag for an adapter
func (app webApp) UpdateAdapterV1(ctx context.Context, input *struct {
	Id   int `path:"id" doc:"the Id of the adapter to update"`
	Body struct {
		ImageTag string `json:"imageTag" doc:"the new image tag"`
	} `body:""`
}) (*struct {
	Body models.Adapter
}, error) {
	if err := requireAdaptersModify(ctx); err != nil {
		return nil, err
	}
	if input.Body.ImageTag == "" {
		return nil, huma.Error400BadRequest("imageTag is required")
	}
	updateQuery := "UPDATE adapters SET imageTag = ? WHERE id = ?"
	result, err := app.db.ExecContext(ctx, updateQuery, input.Body.ImageTag, input.Id)
	if err != nil {
		logging.Error("Database error when updating adapter", ctx, map[string]any{"ERROR": err.Error()})
		return nil, huma.Error500InternalServerError("Internal Server Error")
	}
	rowsAffected, err := result.RowsAffected()
	if err != nil {
		logging.Error("Database error when checking adapter update result", ctx, map[string]any{"ERROR": err.Error()})
		return nil, huma.Error500InternalServerError("Internal Server Error")
	}
	if rowsAffected == 0 {
		return nil, huma.Error404NotFound("adapter not found")
	}
	updatedAdapters, err := app.getAdaptersV1(ctx, &input.Id)
	if err != nil {
		return nil, err
	}
	if len(updatedAdapters) == 0 {
		return nil, huma.Error404NotFound("adapter not found")
	}
	return &struct {
		Body models.Adapter
	}{
		Body: updatedAdapters[0],
	}, nil
}

// GetAdapterAddressV1 returns the service address for a synced adapter
func (app webApp) GetAdapterAddressV1(ctx context.Context, input *struct {
	Id int `path:"id" doc:"the Id of the adapter to retrieve the address for"`
}) (*struct {
	Body struct {
		Address string `json:"address"`
	}
}, error) {
	if err := requireAdaptersViewIfPresent(ctx); err != nil {
		return nil, err
	}
	adapters, err := app.getAdaptersV1(ctx, &input.Id)
	if err != nil {
		return nil, err
	}
	if len(adapters) == 0 {
		return nil, huma.Error404NotFound("adapter not found")
	}
	adapter := adapters[0]
	if adapter.Synced == nil {
		return nil, huma.Error409Conflict("adapter not synced")
	}
	// FIXME should probably not assume this address.
	// Works for now...
	address := fmt.Sprintf("http://adapter-%d.%s:8080", adapter.ID, config.Loaded.ClusterConfig.NameSpace)
	return &struct {
		Body struct {
			Address string `json:"address"`
		}
	}{
		Body: struct {
			Address string `json:"address"`
		}{
			Address: address,
		},
	}, nil
}

// GetAdapterArgumentsForAdapterV1 returns adapter configuration entries
func (app webApp) GetAdapterArgumentsForAdapterV1(ctx context.Context, input *struct {
	Id int `path:"id" doc:"the Id of the adapter to retrieve configuration for"`
}) (*struct {
	Body []models.AdapterConfiguration
}, error) {
	if err := requireAdaptersView(ctx); err != nil {
		return nil, err
	}
	configurations, err := app.getAdapterArgumentsV1(ctx, input.Id)
	if err != nil {
		return nil, err
	}
	return &struct {
		Body []models.AdapterConfiguration
	}{
		Body: configurations,
	}, nil
}

// getAdapterArgumentsV1 is a helper function to get adapter configuration entries
// Returns an API friendly error
func (app webApp) getAdapterArgumentsV1(ctx context.Context, adapterID int) ([]models.AdapterConfiguration, error) {
	query := "SELECT id, adapterId, configKey, configValue, created, updated FROM adapterConfiguration WHERE adapterId = ?"
	rows, err := app.db.QueryContext(ctx, query, adapterID)
	if err != nil {
		logging.Error("Database error when fetching adapter arguments", ctx, map[string]any{"ERROR": err.Error()})
		return nil, huma.Error500InternalServerError("Internal Server Error")
	}
	defer rows.Close()
	result := []models.AdapterConfiguration{}
	for rows.Next() {
		var config models.AdapterConfiguration
		if err := rows.Scan(&config.ID, &config.AdapterID, &config.ConfigKey, &config.ConfigValue, &config.Created, &config.Updated); err != nil {
			logging.Error("Database error when scanning adapter arguments", ctx, map[string]any{"ERROR": err.Error()})
			return nil, huma.Error500InternalServerError("Internal Server Error")
		}
		result = append(result, config)
	}
	if err := rows.Err(); err != nil {
		logging.Error("Database error when iterating adapter arguments", ctx, map[string]any{"ERROR": err.Error()})
		return nil, huma.Error500InternalServerError("Internal Server Error")
	}
	return result, nil
}

// PostAdapterArgumentsForAdapterV1 creates an adapter configuration entry
func (app webApp) PostAdapterArgumentsForAdapterV1(ctx context.Context, input *struct {
	Id   int                         `path:"id" doc:"the Id of the adapter to add configuration for"`
	Body models.AdapterConfiguration `body:""`
}) (*struct {
	Body models.AdapterConfiguration
}, error) {
	if err := requireAdaptersModify(ctx); err != nil {
		return nil, err
	}
	query := `INSERT IGNORE INTO adapterConfiguration (adapterId, configKey, configValue)
			  VALUES (?, ?, ?)
			  RETURNING id, adapterId, configKey, configValue, created, updated`
	row := app.db.QueryRowContext(ctx, query, input.Id, input.Body.ConfigKey, input.Body.ConfigValue)
	var resultConfig models.AdapterConfiguration
	err := row.Scan(&resultConfig.ID, &resultConfig.AdapterID, &resultConfig.ConfigKey, &resultConfig.ConfigValue, &resultConfig.Created, &resultConfig.Updated)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, huma.Error409Conflict("adapter configuration conflict")
		}
		logging.Error("Database error when inserting adapter configuration", ctx, map[string]any{"ERROR": err.Error()})
		return nil, huma.Error500InternalServerError("Internal Server Error")
	}
	return &struct {
		Body models.AdapterConfiguration
	}{
		Body: resultConfig,
	}, nil
}

// DeleteAdapterArgumentsForAdapterV1 deletes an adapter configuration entry
func (app webApp) DeleteAdapterArgumentsForAdapterV1(ctx context.Context, input *struct {
	Id         int `path:"id" doc:"the Id of the adapter"`
	ArgumentId int `path:"argumentId" doc:"the Id of the configuration entry to delete"`
}) (*struct {
}, error) {
	if err := requireAdaptersModify(ctx); err != nil {
		return nil, err
	}
	query := "DELETE FROM adapterConfiguration WHERE id = ? AND adapterId = ?"
	result, err := app.db.ExecContext(ctx, query, input.ArgumentId, input.Id)
	if err != nil {
		logging.Error("Database error when deleting adapter configuration", ctx, map[string]any{"ERROR": err.Error()})
		return nil, huma.Error500InternalServerError("Internal Server Error")
	}
	rowsAffected, err := result.RowsAffected()
	if err != nil {
		logging.Error("Database error when checking adapter configuration delete result", ctx, map[string]any{"ERROR": err.Error()})
		return nil, huma.Error500InternalServerError("Internal Server Error")
	}
	if rowsAffected == 0 {
		return nil, huma.Error404NotFound("adapter configuration not found")
	}
	return nil, nil
}

// PatchAdapterArgumentsForAdapterV1 updates an adapter configuration entry
func (app webApp) PatchAdapterArgumentsForAdapterV1(ctx context.Context, input *struct {
	ArgumentId int                         `path:"argumentId" doc:"the Id of the adapter"`
	AdapterId  int                         `path:"adapterId" doc:"the Id of the configuration entry to update"`
	Body       models.AdapterConfiguration `body:""`
}) (*struct {
	Body models.AdapterConfiguration
}, error) {
	if err := requireAdaptersModify(ctx); err != nil {
		return nil, err
	}
	updateQuery := `UPDATE adapterConfiguration
				   SET configKey = ?, configValue = ?
				   WHERE adapterId = ? AND id = ?`
	result, err := app.db.ExecContext(ctx, updateQuery, input.Body.ConfigKey, input.Body.ConfigValue, input.AdapterId, input.ArgumentId)
	if err != nil {
		logging.Error("Database error when updating adapter configuration", ctx, map[string]any{"ERROR": err.Error()})
		return nil, huma.Error500InternalServerError("Internal Server Error")
	}
	rowsAffected, err := result.RowsAffected()
	if err != nil {
		logging.Error("Database error when checking adapter configuration update result", ctx, map[string]any{"ERROR": err.Error()})
		return nil, huma.Error500InternalServerError("Internal Server Error")
	}
	if rowsAffected == 0 {
		return nil, huma.Error404NotFound("adapter configuration not found")
	}
	selectQuery := "SELECT id, adapterId, configKey, configValue, created, updated FROM adapterConfiguration WHERE adapterId = ? AND id = ?"
	row := app.db.QueryRowContext(ctx, selectQuery, input.AdapterId, input.ArgumentId)
	var resultConfig models.AdapterConfiguration
	if err := row.Scan(&resultConfig.ID, &resultConfig.AdapterID, &resultConfig.ConfigKey, &resultConfig.ConfigValue, &resultConfig.Created, &resultConfig.Updated); err != nil {
		logging.Error("Database error when fetching updated adapter configuration", ctx, map[string]any{"ERROR": err.Error()})
		return nil, huma.Error500InternalServerError("Internal Server Error")
	}
	return &struct {
		Body models.AdapterConfiguration
	}{
		Body: resultConfig,
	}, nil
}

// registerSynced updates a device with information about being synced.
// This is an internal function and does not return API friendly errors.
func (app webApp) registerAdapterSynced(ctx context.Context, adapterId int) error {
	// FIXME allow passing in sync time in order to avoid time skew issues
	updateQuery := "UPDATE adapters SET synced = NOW() WHERE id = ?"
	_, err := app.db.ExecContext(ctx, updateQuery, adapterId)
	return err
}
