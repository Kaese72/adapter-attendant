package restwebapp

import (
	"context"
	"crypto/subtle"
	"database/sql"
	"encoding/json"
	"fmt"
	"strconv"
	"strings"

	"github.com/Kaese72/adapter-attendant/internal/config"
	"github.com/Kaese72/adapter-attendant/internal/logging"
	"github.com/Kaese72/adapter-attendant/rest/models"
	"github.com/Kaese72/authentication/usertoken"
	// aliased: getAdaptersV1 (below, same file) builds its SQL as a local
	// "query" variable, which would otherwise shadow the package.
	libquery "github.com/Kaese72/huemie-lib/query"
	"github.com/danielgtaylor/huma/v2"
)

// adapterFilters defines what filters are available for the adapters model.
var adapterFilters = map[string]libquery.FieldSpec{
	"name":      libquery.Merge(libquery.TextOperators("name")),
	"imageName": libquery.Merge(libquery.TextOperators("imageName")),
	"imageTag":  libquery.Merge(libquery.TextOperators("imageTag")),
	"created":   libquery.Merge(libquery.DateOperators("created")),
	"updated":   libquery.Merge(libquery.DateOperators("updated")),
}

// adapterSortFields are the fields "sort" may reference for GetAdaptersV1.
var adapterSortFields = map[string]string{
	"id":        "id",
	"name":      "name",
	"imageName": "imageName",
	"imageTag":  "imageTag",
	"created":   "created",
	"updated":   "updated",
}

// paginationClause returns the SQL "LIMIT ? OFFSET ?" fragment and its
// arguments for the given pagination. A zero Limit means unbounded, in which
// case no clause is applied (used by internal single-id lookups below).
func paginationClause(pagination libquery.Pagination) (string, []any) {
	if pagination.Limit <= 0 {
		return "", nil
	}
	offset := pagination.Offset
	if offset < 0 {
		offset = 0
	}
	return " LIMIT ? OFFSET ?", []any{pagination.Limit, offset}
}

// countAdapters executes "SELECT COUNT(*) FROM adapters [WHERE <whereClause>]"
// and returns the total number of matching rows, ignoring pagination.
func countAdapters(ctx context.Context, db *sql.DB, whereClause string, args []any) (int, error) {
	q := "SELECT COUNT(*) FROM adapters"
	if whereClause != "" {
		q += " WHERE " + whereClause
	}
	var total int
	row := db.QueryRowContext(ctx, q, args...)
	if err := row.Scan(&total); err != nil {
		return 0, err
	}
	return total, nil
}

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
	db           *sql.DB
	pluginTokens []string
}

func NewWebApp(db *sql.DB, pluginTokens []string) webApp {
	return webApp{
		db:           db,
		pluginTokens: pluginTokens,
	}
}

// GetAdaptersV1 returns a page of adapters
func (app webApp) GetAdaptersV1(ctx context.Context, input *struct {
	Filters string `query:"filters" doc:"a string JSON array of objects containing field, operator, and value for filtering"`
	Sort    string `query:"sort" doc:"a string JSON array of objects containing field and direction ('asc' or 'desc') for sorting"`
	libquery.Pagination
}) (*struct {
	libquery.TotalCount
	Body []models.Adapter
}, error) {
	if err := requireAdaptersView(ctx); err != nil {
		return nil, err
	}
	filters, err := libquery.ParseFilters(input.Filters)
	if err != nil {
		return nil, err
	}
	sorts, err := libquery.ParseSort(input.Sort)
	if err != nil {
		return nil, err
	}
	retAdapters, total, err := app.getAdaptersV1(ctx, nil, filters, sorts, libquery.Pagination{Offset: input.Offset, Limit: input.Limit})
	if err != nil {
		return nil, err
	}
	return &struct {
		libquery.TotalCount
		Body []models.Adapter
	}{
		TotalCount: libquery.TotalCount{TotalCount: total},
		Body:       retAdapters,
	}, nil
}

// getAdaptersV1 is a helper function to get adapters, optionally scoped to a
// single id (in which case filters/sorts/pagination are ignored by callers -
// they're expected to pass nil, nil, libquery.Pagination{}). Returns an API
// friendly error, plus the total number of matching adapters (ignoring
// pagination) for the id == nil, listing case.
func (app webApp) getAdaptersV1(ctx context.Context, id *int, filters []libquery.Filter, sorts []libquery.Sort, pagination libquery.Pagination) ([]models.Adapter, int, error) {
	retAdapters := []models.Adapter{}
	fragments, args, err := libquery.Translate(filters, adapterFilters)
	if err != nil {
		return nil, 0, err
	}
	if id != nil {
		fragments = append(fragments, "id = ?")
		args = append(args, *id)
	}
	whereClause := strings.Join(fragments, " AND ")
	total, err := countAdapters(ctx, app.db, whereClause, args)
	if err != nil {
		logging.Error("Database error when counting adapters", ctx, map[string]interface{}{"ERROR": err.Error()})
		return nil, 0, huma.Error500InternalServerError("Internal Server Error")
	}
	orderBy, err := libquery.BuildOrderBy(sorts, adapterSortFields, "id")
	if err != nil {
		return nil, 0, err
	}
	query := "SELECT id, name, imageName, imageTag, created, updated, synced FROM adapters"
	if whereClause != "" {
		query += " WHERE " + whereClause
	}
	query += " ORDER BY " + orderBy
	limitClause, limitArgs := paginationClause(pagination)
	query += limitClause
	rows, err := app.db.QueryContext(ctx, query, append(append([]any{}, args...), limitArgs...)...)
	if err != nil {
		logging.Error("Database error when fetching adapters", ctx, map[string]interface{}{"ERROR": err.Error()})
		return nil, 0, huma.Error500InternalServerError("Internal Server Error")
	}
	defer rows.Close()
	for rows.Next() {
		var retAdapter models.Adapter
		err := rows.Scan(&retAdapter.ID, &retAdapter.Name, &retAdapter.ImageName, &retAdapter.ImageTag, &retAdapter.Created, &retAdapter.Updated, &retAdapter.Synced)
		if err != nil {
			logging.Error("Database error when fetching adapter", ctx, map[string]interface{}{"ERROR": err.Error()})
			return nil, 0, huma.Error500InternalServerError("Internal Server Error")
		}
		retAdapters = append(retAdapters, retAdapter)
	}
	return retAdapters, total, nil
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
	retAdapters, _, err := app.getAdaptersV1(ctx, &input.Id, nil, nil, libquery.Pagination{})
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
	// The plugin endpoint no longer lists the adapter, so ArgoCD removes its
	// Deployment, Service and Secret.
	return nil, nil
}

// SyncAdapterV1 deploys the adapter as it is configured now. It records the
// image and the adapter's whole environment, with a freshly minted enroll
// token, after which the plugin endpoint lists the adapter and ArgoCD
// deploys (or restarts) it. Edits made since are not deployed until the next
// sync.
func (app webApp) SyncAdapterV1(ctx context.Context, input *struct {
	Id int `path:"id" doc:"the Id of the adapter to sync"`
}) (*struct {
}, error) {
	if err := requireAdaptersModify(ctx); err != nil {
		return nil, err
	}
	syncAdapters, _, err := app.getAdaptersV1(ctx, &input.Id, nil, nil, libquery.Pagination{})
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
	environment, err := adapterEnvironment(syncAdapter.ID, syncArguments)
	if err != nil {
		logging.Error("Error syncing adapter", ctx, map[string]any{"ERROR": err.Error(), "ADAPTER_ID": syncAdapter.ID, "ADAPTER_NAME": syncAdapter.Name})
		return nil, huma.Error500InternalServerError("Internal Server Error")
	}
	err = app.registerAdapterSynced(ctx, syncAdapter.ID, syncAdapter.ImageName+":"+syncAdapter.ImageTag, environment)
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
	updatedAdapters, _, err := app.getAdaptersV1(ctx, &input.Id, nil, nil, libquery.Pagination{})
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
	adapters, _, err := app.getAdaptersV1(ctx, &input.Id, nil, nil, libquery.Pagination{})
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
	address := fmt.Sprintf("http://%s.%s:8080", adapterResourceName(adapter.ID), config.Loaded.ClusterConfig.NameSpace)
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

// registerSynced updates a device with information about being synced,
// including the image and environment the plugin endpoint now hands to
// ArgoCD.
// This is an internal function and does not return API friendly errors.
func (app webApp) registerAdapterSynced(ctx context.Context, adapterId int, image string, environment map[string]string) error {
	encodedEnvironment, err := json.Marshal(environment)
	if err != nil {
		return err
	}
	// FIXME allow passing in sync time in order to avoid time skew issues
	updateQuery := "UPDATE adapters SET synced = NOW(), syncedImage = ?, syncedEnvironment = ?, syncGeneration = syncGeneration + 1 WHERE id = ?"
	_, err = app.db.ExecContext(ctx, updateQuery, image, string(encodedEnvironment), adapterId)
	return err
}

func (app webApp) checkPluginToken(authHeader string) bool {
	provided, ok := strings.CutPrefix(authHeader, "Bearer ")
	if !ok || provided == "" {
		return false
	}
	matched := false
	for _, token := range app.pluginTokens {
		if subtle.ConstantTimeCompare([]byte(provided), []byte(token)) == 1 {
			matched = true
		}
	}
	return matched
}

// PluginGetParams implements ArgoCD's ApplicationSet Plugin generator
// protocol (POST .../api/v1/getparams.execute), the same way
// appliance-registry does for cloud-connect-server - see huemie-gitops-base's
// appliance/argocd/applicationset-adapters.yaml. It answers with one
// parameter set per adapter synced since adapters were deployed by ArgoCD,
// including the adapter's credentials; ArgoCD deploys exactly those, and
// removes the rest.
//
// An empty answer is never given for a failure: ArgoCD would delete every
// adapter's Application for it. A failure is a 500, on which ArgoCD keeps the
// Applications as they are.
func (app webApp) PluginGetParams(ctx context.Context, input *struct {
	Authorization string `header:"Authorization"`
	Body          models.ArgoCDPluginGetParamsRequest
}) (*struct {
	Body models.ArgoCDPluginGetParamsResponse
}, error) {
	if !app.checkPluginToken(input.Authorization) {
		return nil, huma.Error401Unauthorized("invalid plugin token")
	}
	rows, err := app.db.QueryContext(ctx, "SELECT id, syncedImage, syncedEnvironment, syncGeneration FROM adapters WHERE syncedImage IS NOT NULL AND syncedEnvironment IS NOT NULL ORDER BY id")
	if err != nil {
		logging.Error("Database error when fetching synced adapters", ctx, map[string]any{"ERROR": err.Error()})
		return nil, huma.Error500InternalServerError("Internal Server Error")
	}
	defer rows.Close()
	resp := models.ArgoCDPluginGetParamsResponse{}
	resp.Output.Parameters = []models.ArgoCDPluginParameter{}
	for rows.Next() {
		var id int
		var image string
		var environment string
		var syncGeneration uint64
		if err := rows.Scan(&id, &image, &environment, &syncGeneration); err != nil {
			logging.Error("Database error when scanning synced adapters", ctx, map[string]any{"ERROR": err.Error()})
			return nil, huma.Error500InternalServerError("Internal Server Error")
		}
		// Skipping just this adapter would make ArgoCD delete it, so a
		// broken one fails the whole answer instead.
		data, err := secretData(environment)
		if err != nil {
			logging.Error("Error reading synced environment", ctx, map[string]any{"ERROR": err.Error(), "ADAPTER_ID": id})
			return nil, huma.Error500InternalServerError("Internal Server Error")
		}
		resp.Output.Parameters = append(resp.Output.Parameters, models.ArgoCDPluginParameter{
			ID:             strconv.Itoa(id),
			Image:          image,
			SyncGeneration: strconv.FormatUint(syncGeneration, 10),
			SecretData:     data,
		})
	}
	if err := rows.Err(); err != nil {
		logging.Error("Database error when iterating synced adapters", ctx, map[string]any{"ERROR": err.Error()})
		return nil, huma.Error500InternalServerError("Internal Server Error")
	}
	return &struct {
		Body models.ArgoCDPluginGetParamsResponse
	}{Body: resp}, nil
}
