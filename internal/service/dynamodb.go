package service

import (
	"context"
	"encoding/json"
	"fmt"
	"reflect"
	"sort"
	"strings"

	"github.com/dostrow/e9s/internal/model"
)

// DynamoDBAPI is the low-level DynamoDB behavior shared by both frontends.
type DynamoDBAPI interface {
	ListDynamoTables(context.Context, string) ([]string, error)
	DescribeDynamoTable(context.Context, string) (*model.DynamoTable, error)
	ScanDynamoTable(context.Context, string, int, string) (*model.DynamoPage, error)
	ScanDynamoTableWithFilter(context.Context, string, string, string, string, int, string) (*model.DynamoPage, error)
	ScanDynamoTableWithFuncFilter(context.Context, string, string, string, string, int, string) (*model.DynamoPage, error)
	ExecutePartiQL(context.Context, string) ([]model.DynamoItem, error)
	GetDynamoItem(context.Context, string, model.DynamoItem) (*model.DynamoItem, error)
	UpdateDynamoField(context.Context, string, model.DynamoItem, string, any, string) error
	PutDynamoItem(context.Context, string, model.DynamoItem, []string) error
}

type DynamoDB struct{ api DynamoDBAPI }

func NewDynamoDB(api DynamoDBAPI) *DynamoDB { return &DynamoDB{api: api} }

func (s *DynamoDB) Tables(ctx context.Context, filter string) ([]string, error) {
	tables, err := s.api.ListDynamoTables(ctx, strings.TrimSpace(filter))
	if err != nil {
		return nil, fmt.Errorf("list DynamoDB tables: %w", err)
	}
	sort.SliceStable(tables, func(i, j int) bool {
		return strings.ToLower(tables[i]) < strings.ToLower(tables[j])
	})
	return tables, nil
}

func (s *DynamoDB) Table(ctx context.Context, table string) (*model.DynamoTable, error) {
	table = strings.TrimSpace(table)
	if table == "" {
		return nil, fmt.Errorf("read DynamoDB table: table name is required")
	}
	detail, err := s.api.DescribeDynamoTable(ctx, table)
	if err != nil {
		return nil, fmt.Errorf("read DynamoDB table %q: %w", table, err)
	}
	if detail == nil {
		return nil, fmt.Errorf("read DynamoDB table %q: table was not found", table)
	}
	return detail, nil
}

func (s *DynamoDB) Scan(ctx context.Context, request model.DynamoScanRequest) (*model.DynamoPage, error) {
	request.Table = strings.TrimSpace(request.Table)
	if request.Table == "" {
		return nil, fmt.Errorf("scan DynamoDB table: table name is required")
	}
	if request.Limit <= 0 {
		request.Limit = 50
	}
	var (
		page *model.DynamoPage
		err  error
	)
	if request.Filter == nil {
		page, err = s.api.ScanDynamoTable(ctx, request.Table, request.Limit, request.NextToken)
	} else {
		filter := *request.Filter
		filter.Attribute = strings.TrimSpace(filter.Attribute)
		if filter.Attribute == "" {
			return nil, fmt.Errorf("scan DynamoDB table %q: filter attribute is required", request.Table)
		}
		switch filter.Operator {
		case "begins_with", "contains":
			page, err = s.api.ScanDynamoTableWithFuncFilter(ctx, request.Table, filter.Attribute, filter.Operator, filter.Value, request.Limit, request.NextToken)
		case "=", "<>", "<", "<=", ">", ">=":
			page, err = s.api.ScanDynamoTableWithFilter(ctx, request.Table, filter.Attribute, filter.Operator, filter.Value, request.Limit, request.NextToken)
		default:
			return nil, fmt.Errorf("scan DynamoDB table %q: unsupported filter operator %q", request.Table, filter.Operator)
		}
	}
	if err != nil {
		return nil, fmt.Errorf("scan DynamoDB table %q: %w", request.Table, err)
	}
	return page, nil
}

func (s *DynamoDB) PartiQL(ctx context.Context, statement string) ([]model.DynamoItem, error) {
	statement = strings.TrimSpace(statement)
	if statement == "" {
		return nil, fmt.Errorf("execute DynamoDB PartiQL: statement is required")
	}
	items, err := s.api.ExecutePartiQL(ctx, statement)
	if err != nil {
		return nil, fmt.Errorf("execute DynamoDB PartiQL: %w", err)
	}
	return items, nil
}

func (s *DynamoDB) Item(ctx context.Context, table string, keyNames []string, item model.DynamoItem) (*model.DynamoItem, error) {
	key, err := BuildDynamoKey(item, keyNames)
	if err != nil {
		return nil, err
	}
	result, err := s.api.GetDynamoItem(ctx, table, key)
	if err != nil {
		return nil, fmt.Errorf("read DynamoDB item from %q: %w", table, err)
	}
	if result == nil {
		return nil, fmt.Errorf("read DynamoDB item from %q: item was not found", table)
	}
	return result, nil
}

func (s *DynamoDB) UpdateField(ctx context.Context, request model.DynamoFieldUpdate) error {
	if strings.TrimSpace(request.Table) == "" || request.Attribute == "" {
		return fmt.Errorf("update DynamoDB item: table and attribute are required")
	}
	for _, keyName := range request.KeyNames {
		if request.Attribute == keyName {
			return fmt.Errorf("update DynamoDB item: key attribute %q cannot be edited", keyName)
		}
	}
	key, err := BuildDynamoKey(request.Item, request.KeyNames)
	if err != nil {
		return err
	}
	if err := s.api.UpdateDynamoField(ctx, request.Table, key, request.Attribute, request.OriginalValue, request.NewValue); err != nil {
		return fmt.Errorf("update DynamoDB item in %q: %w", request.Table, err)
	}
	return nil
}

func (s *DynamoDB) PutItem(ctx context.Context, request model.DynamoPutRequest) error {
	request.Table = strings.TrimSpace(request.Table)
	if request.Table == "" || len(request.Item) == 0 {
		return fmt.Errorf("write DynamoDB item: table and item are required")
	}
	if _, err := BuildDynamoKey(request.Item, request.KeyNames); err != nil {
		return err
	}
	if err := s.api.PutDynamoItem(ctx, request.Table, request.Item, request.KeyNames); err != nil {
		return fmt.Errorf("write DynamoDB item to %q: %w", request.Table, err)
	}
	return nil
}

func BuildDynamoKey(item model.DynamoItem, keyNames []string) (model.DynamoItem, error) {
	if len(keyNames) == 0 {
		return nil, fmt.Errorf("build DynamoDB item key: table key schema is unavailable")
	}
	key := model.DynamoItem{}
	for _, name := range keyNames {
		value, found := item[name]
		if !found {
			return nil, fmt.Errorf("build DynamoDB item key: key attribute %q is missing", name)
		}
		key[name] = value
	}
	return key, nil
}

// DynamoItemsShareKey reports whether two items identify the same DynamoDB
// record under the supplied table key schema.
func DynamoItemsShareKey(left, right model.DynamoItem, keyNames []string) (bool, error) {
	leftKey, err := BuildDynamoKey(left, keyNames)
	if err != nil {
		return false, err
	}
	rightKey, err := BuildDynamoKey(right, keyNames)
	if err != nil {
		return false, err
	}
	return reflect.DeepEqual(leftKey, rightKey), nil
}

func DynamoItemToJSON(item model.DynamoItem) string {
	data, err := json.MarshalIndent(item, "", "  ")
	if err != nil {
		return fmt.Sprintf("%v", item)
	}
	return string(data)
}

func DynamoValueToEditableString(value any) string {
	if text, ok := value.(string); ok {
		return text
	}
	data, err := json.MarshalIndent(value, "", "  ")
	if err == nil {
		return string(data)
	}
	return fmt.Sprintf("%v", value)
}

func ParseDynamoItemJSON(document string) (model.DynamoItem, error) {
	var item model.DynamoItem
	decoder := json.NewDecoder(strings.NewReader(document))
	decoder.UseNumber()
	if err := decoder.Decode(&item); err != nil {
		return nil, fmt.Errorf("parse DynamoDB item JSON: %w", err)
	}
	if len(item) == 0 {
		return nil, fmt.Errorf("parse DynamoDB item JSON: item is empty")
	}
	return item, nil
}
