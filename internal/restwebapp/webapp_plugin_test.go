package restwebapp

import (
	"context"
	"errors"
	"testing"

	"github.com/Kaese72/adapter-attendant/rest/models"
	"github.com/danielgtaylor/huma/v2"
)

func TestCheckPluginToken(t *testing.T) {
	app := webApp{pluginTokens: []string{"old", "new"}}
	cases := map[string]bool{
		"Bearer old":  true,
		"Bearer new":  true,
		"Bearer nope": false,
		"Bearer ":     false,
		"old":         false,
		"":            false,
	}
	for header, want := range cases {
		if got := app.checkPluginToken(header); got != want {
			t.Errorf("checkPluginToken(%q) = %v, want %v", header, got, want)
		}
	}
}

// A bad token must be rejected before the database is touched (db is nil
// here, so reaching it would panic).
func TestPluginGetParamsRejectsBadToken(t *testing.T) {
	app := webApp{pluginTokens: []string{"secret"}}
	_, err := app.PluginGetParams(context.Background(), &struct {
		Authorization string `header:"Authorization"`
		Body          models.ArgoCDPluginGetParamsRequest
	}{Authorization: "Bearer wrong"})
	var statusErr huma.StatusError
	if !errors.As(err, &statusErr) || statusErr.GetStatus() != 401 {
		t.Fatalf("expected 401, got %v", err)
	}
}
