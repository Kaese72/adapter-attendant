package restwebapp

import (
	"testing"

	"github.com/Kaese72/huemie-lib/query"
)

// These exercise adapterFilters/adapterSortFields directly (no DB needed,
// since query.Translate/BuildOrderBy are pure functions) - they catch a
// typo'd column name or operator wiring without needing a database.
func TestAdapterFilters(t *testing.T) {
	t.Run("imageName text-contains", func(t *testing.T) {
		fragments, args, err := query.Translate([]query.Filter{
			{Field: "imageName", Operator: "text-contains", Value: "hue"},
		}, adapterFilters)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if len(fragments) != 1 || len(args) != 1 {
			t.Fatalf("unexpected result: fragments=%v args=%v", fragments, args)
		}
	})

	t.Run("created date-gt", func(t *testing.T) {
		fragments, args, err := query.Translate([]query.Filter{
			{Field: "created", Operator: "date-gt", Value: "2026-09-27T00:00:00Z"},
		}, adapterFilters)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if len(fragments) != 1 || len(args) != 1 {
			t.Fatalf("unexpected result: fragments=%v args=%v", fragments, args)
		}
	})

	t.Run("updated date-lt", func(t *testing.T) {
		fragments, args, err := query.Translate([]query.Filter{
			{Field: "updated", Operator: "date-lt", Value: "2026-09-27T00:00:00Z"},
		}, adapterFilters)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if len(fragments) != 1 || len(args) != 1 {
			t.Fatalf("unexpected result: fragments=%v args=%v", fragments, args)
		}
	})

	t.Run("unknown field", func(t *testing.T) {
		if _, _, err := query.Translate([]query.Filter{{Field: "synced", Operator: "eq", Value: "x"}}, adapterFilters); err == nil {
			t.Fatal("expected an error - synced is not a filterable field")
		}
	})
}

func TestAdapterSortFields(t *testing.T) {
	t.Run("empty falls back to id", func(t *testing.T) {
		clause, err := query.BuildOrderBy(nil, adapterSortFields, "id")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if clause != "id" {
			t.Fatalf("expected fallback \"id\", got %q", clause)
		}
	})

	t.Run("sort by imageName asc", func(t *testing.T) {
		clause, err := query.BuildOrderBy([]query.Sort{{Field: "imageName", Direction: "asc"}}, adapterSortFields, "id")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if clause != "imageName ASC" {
			t.Fatalf("unexpected clause: %q", clause)
		}
	})

	t.Run("sort by imageTag desc", func(t *testing.T) {
		clause, err := query.BuildOrderBy([]query.Sort{{Field: "imageTag", Direction: "desc"}}, adapterSortFields, "id")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if clause != "imageTag DESC" {
			t.Fatalf("unexpected clause: %q", clause)
		}
	})

	t.Run("sort by created/updated", func(t *testing.T) {
		if _, err := query.BuildOrderBy([]query.Sort{{Field: "created", Direction: "asc"}}, adapterSortFields, "id"); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if _, err := query.BuildOrderBy([]query.Sort{{Field: "updated", Direction: "asc"}}, adapterSortFields, "id"); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
	})
}
