//go:build gui

package gui

import (
	"reflect"
	"strings"
	"testing"

	"github.com/dostrow/e9s/internal/config"
	"github.com/dostrow/e9s/internal/model"
)

func TestFilterDynamoTablesIsCaseInsensitive(t *testing.T) {
	tables := []string{"Prod-Users", "archive", "prod-events"}
	if got := filterDynamoTables(tables, " PROD "); !reflect.DeepEqual(got, []string{"Prod-Users", "prod-events"}) {
		t.Fatalf("filterDynamoTables() = %#v", got)
	}
}

func TestDynamoBreadcrumbsPreserveSavedDestinations(t *testing.T) {
	if got := dynamoBreadcrumb("Production", "users"); got != "DynamoDB / Production / users" {
		t.Fatalf("dynamoBreadcrumb() = %q", got)
	}
	if got := dynamoItemBreadcrumb("", "Recent failures", ""); got != "DynamoDB / Recent failures" {
		t.Fatalf("dynamoItemBreadcrumb() = %q", got)
	}
}

func TestFormatDynamoTableAndItem(t *testing.T) {
	table := &model.DynamoTable{
		Name: "users", Status: "ACTIVE", ItemCount: 12, BillingMode: "PAY_PER_REQUEST",
		KeySchema: []model.DynamoKeyElement{{Name: "pk", Type: "HASH", AttrType: "S"}},
		GSIs:      []string{"by-email"},
	}
	if got := formatDynamoTable(table); !strings.Contains(got, "PAY_PER_REQUEST") || !strings.Contains(got, "by-email") {
		t.Fatalf("formatDynamoTable() = %q", got)
	}
	item := model.DynamoItem{"pk": "user#1", "active": true}
	if got := formatDynamoItem(item, []string{"pk"}); !strings.Contains(got, "pk = user#1") || !strings.Contains(got, `"active": true`) {
		t.Fatalf("formatDynamoItem() = %q", got)
	}
}

func TestCompactDynamoItemIsDeterministic(t *testing.T) {
	item := model.DynamoItem{"z": 2, "a": 1}
	if got := compactDynamoItem(item); got != `{"a":1,"z":2}` {
		t.Fatalf("compactDynamoItem() = %q", got)
	}
}

func TestDynamoItemListSummaryReportsCumulativeScanAndPagination(t *testing.T) {
	got := dynamoItemListSummary("events", 73, 150, true)
	for _, want := range []string{"Loaded   73", "Scanned  150", "More items are available"} {
		if !strings.Contains(got, want) {
			t.Fatalf("dynamoItemListSummary() = %q, missing %q", got, want)
		}
	}
}

func TestFindDynamoQuery(t *testing.T) {
	queries := []config.DynamoQuery{{Name: "recent", Statement: "SELECT * FROM events"}}
	query, found := findDynamoQuery(queries, "recent")
	if !found || query.Statement != "SELECT * FROM events" {
		t.Fatalf("findDynamoQuery() = %#v, %v", query, found)
	}
	if _, found := findDynamoQuery(queries, "missing"); found {
		t.Fatal("findDynamoQuery() unexpectedly found missing query")
	}
}

func TestEditableDynamoAttributesExcludesKeysAndSorts(t *testing.T) {
	item := model.DynamoItem{"z": 1, "pk": "one", "a": 2, "sk": "two"}
	got := editableDynamoAttributes(item, []string{"pk", "sk"})
	if !reflect.DeepEqual(got, []string{"a", "z"}) {
		t.Fatalf("editableDynamoAttributes() = %#v", got)
	}
}

func TestDynamoItemsShareKeyIgnoresNonKeyFields(t *testing.T) {
	left := model.DynamoItem{"pk": "one", "value": "before"}
	right := model.DynamoItem{"pk": "one", "value": "after"}
	if !dynamoItemsShareKey(left, right, []string{"pk"}) {
		t.Fatal("dynamoItemsShareKey() did not match equal keys")
	}
	right["pk"] = "two"
	if dynamoItemsShareKey(left, right, []string{"pk"}) {
		t.Fatal("dynamoItemsShareKey() matched different keys")
	}
}
