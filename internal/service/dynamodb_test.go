package service

import (
	"context"
	"errors"
	"reflect"
	"strings"
	"testing"

	"github.com/dostrow/e9s/internal/model"
)

type fakeDynamoDBAPI struct {
	tables       []string
	table        *model.DynamoTable
	page         *model.DynamoPage
	item         *model.DynamoItem
	err          error
	scanKind     string
	scanTable    string
	scanOperator string
	scanToken    string
	updatedKey   model.DynamoItem
	putItem      model.DynamoItem
}

func (f *fakeDynamoDBAPI) ListDynamoTables(context.Context, string) ([]string, error) {
	return append([]string(nil), f.tables...), f.err
}
func (f *fakeDynamoDBAPI) DescribeDynamoTable(context.Context, string) (*model.DynamoTable, error) {
	return f.table, f.err
}
func (f *fakeDynamoDBAPI) ScanDynamoTable(_ context.Context, table string, _ int, token string) (*model.DynamoPage, error) {
	f.scanKind, f.scanTable, f.scanToken = "scan", table, token
	return f.page, f.err
}
func (f *fakeDynamoDBAPI) ScanDynamoTableWithFilter(_ context.Context, table, _, operator, _ string, _ int, token string) (*model.DynamoPage, error) {
	f.scanKind, f.scanTable, f.scanOperator, f.scanToken = "comparison", table, operator, token
	return f.page, f.err
}
func (f *fakeDynamoDBAPI) ScanDynamoTableWithFuncFilter(_ context.Context, table, _, operator, _ string, _ int, token string) (*model.DynamoPage, error) {
	f.scanKind, f.scanTable, f.scanOperator, f.scanToken = "function", table, operator, token
	return f.page, f.err
}
func (f *fakeDynamoDBAPI) ExecutePartiQL(context.Context, string) ([]model.DynamoItem, error) {
	if f.page == nil {
		return nil, f.err
	}
	return f.page.Items, f.err
}
func (f *fakeDynamoDBAPI) GetDynamoItem(context.Context, string, model.DynamoItem) (*model.DynamoItem, error) {
	return f.item, f.err
}
func (f *fakeDynamoDBAPI) UpdateDynamoField(_ context.Context, _ string, key model.DynamoItem, _ string, _ any, _ string) error {
	f.updatedKey = key
	return f.err
}
func (f *fakeDynamoDBAPI) PutDynamoItem(_ context.Context, _ string, item model.DynamoItem) error {
	f.putItem = item
	return f.err
}

func TestDynamoDBTablesSortAndWrapErrors(t *testing.T) {
	api := &fakeDynamoDBAPI{tables: []string{"zeta", "Alpha", "beta"}}
	tables, err := NewDynamoDB(api).Tables(context.Background(), "")
	if err != nil || !reflect.DeepEqual(tables, []string{"Alpha", "beta", "zeta"}) {
		t.Fatalf("Tables() = %#v, %v", tables, err)
	}
	api.err = errors.New("denied")
	if _, err := NewDynamoDB(api).Tables(context.Background(), ""); err == nil || !strings.Contains(err.Error(), "list DynamoDB tables") {
		t.Fatalf("Tables() error = %v", err)
	}
}

func TestDynamoDBScanRoutesOperatorsAndTokens(t *testing.T) {
	api := &fakeDynamoDBAPI{page: &model.DynamoPage{NextToken: "next"}}
	service := NewDynamoDB(api)
	page, err := service.Scan(context.Background(), model.DynamoScanRequest{
		Table: "events", Limit: 20, NextToken: "cursor",
		Filter: &model.DynamoFilter{Attribute: "kind", Operator: "contains", Value: "error"},
	})
	if err != nil || page.NextToken != "next" || api.scanKind != "function" || api.scanToken != "cursor" {
		t.Fatalf("function Scan() = %#v, %v; api = %#v", page, err, api)
	}
	_, err = service.Scan(context.Background(), model.DynamoScanRequest{
		Table: "events", Filter: &model.DynamoFilter{Attribute: "kind", Operator: ">=", Value: "a"},
	})
	if err != nil || api.scanKind != "comparison" || api.scanOperator != ">=" {
		t.Fatalf("comparison Scan() error = %v; api = %#v", err, api)
	}
}

func TestDynamoDBMutationBuildsKeysAndRejectsKeyEdits(t *testing.T) {
	api := &fakeDynamoDBAPI{}
	service := NewDynamoDB(api)
	item := model.DynamoItem{"pk": "one", "sk": "two", "value": "old"}
	request := model.DynamoFieldUpdate{
		Table: "items", KeyNames: []string{"pk", "sk"}, Item: item,
		Attribute: "value", OriginalValue: "old", NewValue: "new",
	}
	if err := service.UpdateField(context.Background(), request); err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(api.updatedKey, model.DynamoItem{"pk": "one", "sk": "two"}) {
		t.Fatalf("updated key = %#v", api.updatedKey)
	}
	request.Attribute = "pk"
	if err := service.UpdateField(context.Background(), request); err == nil {
		t.Fatal("UpdateField accepted a key mutation")
	}
}

func TestParseDynamoItemJSONPreservesNumbers(t *testing.T) {
	item, err := ParseDynamoItemJSON(`{"pk":"one","count":12345678901234567890}`)
	if err != nil {
		t.Fatal(err)
	}
	if got := item["count"].(interface{ String() string }).String(); got != "12345678901234567890" {
		t.Fatalf("count = %q", got)
	}
}
