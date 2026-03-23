package aws

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/aws/aws-sdk-go-v2/feature/dynamodb/attributevalue"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb"
	dbtypes "github.com/aws/aws-sdk-go-v2/service/dynamodb/types"
)

type DynamoTable struct {
	Name        string
	Status      string
	ItemCount   int64
	SizeBytes   int64
	BillingMode string
	KeySchema   []DynamoKeyElement
	GSIs        []string
}

type DynamoKeyElement struct {
	Name    string
	Type    string // HASH or RANGE
	AttrType string // S, N, B
}

type DynamoItem map[string]interface{}

type DynamoScanResult struct {
	Items            []DynamoItem
	Count            int
	ScannedCount     int
	LastEvaluatedKey map[string]dbtypes.AttributeValue
}

// ListTables returns DynamoDB table names, optionally filtered by substring.
func (c *Client) ListDynamoTables(ctx context.Context, filter string) ([]string, error) {
	var tables []string
	var lastTable *string

	for {
		input := &dynamodb.ListTablesInput{
			ExclusiveStartTableName: lastTable,
		}
		out, err := c.DynamoDB.ListTables(ctx, input)
		if err != nil {
			return nil, err
		}
		for _, t := range out.TableNames {
			if filter == "" || strings.Contains(strings.ToLower(t), strings.ToLower(filter)) {
				tables = append(tables, t)
			}
		}
		if out.LastEvaluatedTableName == nil {
			break
		}
		lastTable = out.LastEvaluatedTableName
	}
	return tables, nil
}

// DescribeDynamoTable returns detailed info about a table.
func (c *Client) DescribeDynamoTable(ctx context.Context, tableName string) (*DynamoTable, error) {
	out, err := c.DynamoDB.DescribeTable(ctx, &dynamodb.DescribeTableInput{
		TableName: &tableName,
	})
	if err != nil {
		return nil, err
	}

	td := out.Table
	table := &DynamoTable{
		Name:   derefStrAws(td.TableName),
		Status: string(td.TableStatus),
	}
	if td.ItemCount != nil {
		table.ItemCount = *td.ItemCount
	}
	if td.TableSizeBytes != nil {
		table.SizeBytes = *td.TableSizeBytes
	}
	if td.BillingModeSummary != nil {
		table.BillingMode = string(td.BillingModeSummary.BillingMode)
	}

	// Key schema
	attrTypes := map[string]string{}
	for _, ad := range td.AttributeDefinitions {
		if ad.AttributeName != nil {
			attrTypes[*ad.AttributeName] = string(ad.AttributeType)
		}
	}
	for _, ks := range td.KeySchema {
		if ks.AttributeName != nil {
			table.KeySchema = append(table.KeySchema, DynamoKeyElement{
				Name:     *ks.AttributeName,
				Type:     string(ks.KeyType),
				AttrType: attrTypes[*ks.AttributeName],
			})
		}
	}

	for _, gsi := range td.GlobalSecondaryIndexes {
		if gsi.IndexName != nil {
			table.GSIs = append(table.GSIs, *gsi.IndexName)
		}
	}

	return table, nil
}

// ScanDynamoTable scans a table and returns items as generic maps.
func (c *Client) ScanDynamoTable(ctx context.Context, tableName string, limit int, startKey map[string]dbtypes.AttributeValue) (*DynamoScanResult, error) {
	input := &dynamodb.ScanInput{
		TableName: &tableName,
	}
	if limit > 0 {
		l := int32(limit)
		input.Limit = &l
	}
	if len(startKey) > 0 {
		input.ExclusiveStartKey = startKey
	}

	out, err := c.DynamoDB.Scan(ctx, input)
	if err != nil {
		return nil, err
	}

	items := make([]DynamoItem, 0, len(out.Items))
	for _, item := range out.Items {
		var m DynamoItem
		if err := attributevalue.UnmarshalMap(item, &m); err != nil {
			continue
		}
		items = append(items, m)
	}

	return &DynamoScanResult{
		Items:            items,
		Count:            int(out.Count),
		ScannedCount:     int(out.ScannedCount),
		LastEvaluatedKey: out.LastEvaluatedKey,
	}, nil
}

// ScanDynamoTableWithFilter scans with a filter expression.
func (c *Client) ScanDynamoTableWithFilter(ctx context.Context, tableName, attrName, operator, value string, limit int, startKey map[string]dbtypes.AttributeValue) (*DynamoScanResult, error) {
	filterExpr := fmt.Sprintf("#attr %s :val", operator)
	input := &dynamodb.ScanInput{
		TableName:        &tableName,
		FilterExpression: &filterExpr,
		ExpressionAttributeNames: map[string]string{
			"#attr": attrName,
		},
		ExpressionAttributeValues: map[string]dbtypes.AttributeValue{
			":val": &dbtypes.AttributeValueMemberS{Value: value},
		},
	}
	if limit > 0 {
		l := int32(limit)
		input.Limit = &l
	}
	if len(startKey) > 0 {
		input.ExclusiveStartKey = startKey
	}

	out, err := c.DynamoDB.Scan(ctx, input)
	if err != nil {
		return nil, err
	}

	items := make([]DynamoItem, 0, len(out.Items))
	for _, item := range out.Items {
		var m DynamoItem
		if err := attributevalue.UnmarshalMap(item, &m); err != nil {
			continue
		}
		items = append(items, m)
	}

	return &DynamoScanResult{
		Items:            items,
		Count:            int(out.Count),
		ScannedCount:     int(out.ScannedCount),
		LastEvaluatedKey: out.LastEvaluatedKey,
	}, nil
}

// ScanDynamoTableWithFuncFilter scans with a function-style filter (contains, begins_with).
func (c *Client) ScanDynamoTableWithFuncFilter(ctx context.Context, tableName, attrName, funcName, value string, limit int, startKey map[string]dbtypes.AttributeValue) (*DynamoScanResult, error) {
	filterExpr := fmt.Sprintf("%s(#attr, :val)", funcName)
	input := &dynamodb.ScanInput{
		TableName:        &tableName,
		FilterExpression: &filterExpr,
		ExpressionAttributeNames: map[string]string{
			"#attr": attrName,
		},
		ExpressionAttributeValues: map[string]dbtypes.AttributeValue{
			":val": &dbtypes.AttributeValueMemberS{Value: value},
		},
	}
	if limit > 0 {
		l := int32(limit)
		input.Limit = &l
	}
	if len(startKey) > 0 {
		input.ExclusiveStartKey = startKey
	}

	out, err := c.DynamoDB.Scan(ctx, input)
	if err != nil {
		return nil, err
	}

	items := make([]DynamoItem, 0, len(out.Items))
	for _, item := range out.Items {
		var m DynamoItem
		if err := attributevalue.UnmarshalMap(item, &m); err != nil {
			continue
		}
		items = append(items, m)
	}

	return &DynamoScanResult{
		Items:            items,
		Count:            int(out.Count),
		ScannedCount:     int(out.ScannedCount),
		LastEvaluatedKey: out.LastEvaluatedKey,
	}, nil
}

// ExecutePartiQL runs a PartiQL statement and returns results.
func (c *Client) ExecutePartiQL(ctx context.Context, statement string) ([]DynamoItem, error) {
	out, err := c.DynamoDB.ExecuteStatement(ctx, &dynamodb.ExecuteStatementInput{
		Statement: &statement,
	})
	if err != nil {
		return nil, err
	}

	items := make([]DynamoItem, 0, len(out.Items))
	for _, item := range out.Items {
		var m DynamoItem
		if err := attributevalue.UnmarshalMap(item, &m); err != nil {
			continue
		}
		items = append(items, m)
	}
	return items, nil
}

// DynamoItemToJSON converts a DynamoItem to pretty-printed JSON.
func DynamoItemToJSON(item DynamoItem) string {
	b, err := json.MarshalIndent(item, "", "  ")
	if err != nil {
		return fmt.Sprintf("%v", item)
	}
	return string(b)
}
