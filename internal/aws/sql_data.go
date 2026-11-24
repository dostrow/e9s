package aws

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"strconv"
	"time"

	awssdk "github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/rdsdata"
	"github.com/aws/aws-sdk-go-v2/service/rdsdata/types"
	"github.com/dostrow/e9s/internal/model"
)

func (c *Client) ExecuteSQLData(ctx context.Context, request model.SQLQueryRequest) (model.SQLQueryResult, error) {
	if c.RDSData == nil {
		return model.SQLQueryResult{}, fmt.Errorf("RDS Data API client is unavailable")
	}
	started := time.Now()
	input := &rdsdata.ExecuteStatementInput{
		ResourceArn: awssdk.String(request.ResourceARN), SecretArn: awssdk.String(request.SecretARN),
		Database: awssdk.String(request.Database), Sql: awssdk.String(request.SQL), IncludeResultMetadata: true,
	}
	if request.TransactionID != "" {
		input.TransactionId = awssdk.String(request.TransactionID)
	}
	output, err := c.RDSData.ExecuteStatement(ctx, input)
	if err != nil {
		return model.SQLQueryResult{}, err
	}
	result := model.SQLQueryResult{RowsAffected: output.NumberOfRecordsUpdated, DurationMS: time.Since(started).Milliseconds()}
	for _, column := range output.ColumnMetadata {
		name := awssdk.ToString(column.Label)
		if name == "" {
			name = awssdk.ToString(column.Name)
		}
		result.Columns = append(result.Columns, name)
	}
	limit := request.MaxRows
	if limit <= 0 {
		limit = 10000
	}
	for index, record := range output.Records {
		if index >= limit {
			result.Truncated = true
			break
		}
		row := make([]string, len(record))
		for fieldIndex, field := range record {
			row[fieldIndex] = dataAPIFieldString(field)
		}
		result.Rows = append(result.Rows, row)
	}
	if len(result.Columns) > 0 {
		result.CommandTag = fmt.Sprintf("SELECT %d", len(result.Rows))
	} else {
		result.CommandTag = fmt.Sprintf("%d rows affected", result.RowsAffected)
	}
	return result, nil
}

// BeginSQLDataTransaction starts a Data API transaction that later
// statements join through SQLQueryRequest.TransactionID.
func (c *Client) BeginSQLDataTransaction(ctx context.Context, request model.SQLQueryRequest) (string, error) {
	if c.RDSData == nil {
		return "", fmt.Errorf("RDS Data API client is unavailable")
	}
	output, err := c.RDSData.BeginTransaction(ctx, &rdsdata.BeginTransactionInput{
		ResourceArn: awssdk.String(request.ResourceARN), SecretArn: awssdk.String(request.SecretARN),
		Database: awssdk.String(request.Database),
	})
	if err != nil {
		return "", err
	}
	return awssdk.ToString(output.TransactionId), nil
}

// EndSQLDataTransaction commits or rolls back request.TransactionID and
// returns the Data API's transaction status.
func (c *Client) EndSQLDataTransaction(ctx context.Context, request model.SQLQueryRequest, commit bool) (string, error) {
	if c.RDSData == nil {
		return "", fmt.Errorf("RDS Data API client is unavailable")
	}
	if commit {
		output, err := c.RDSData.CommitTransaction(ctx, &rdsdata.CommitTransactionInput{
			ResourceArn: awssdk.String(request.ResourceARN), SecretArn: awssdk.String(request.SecretARN),
			TransactionId: awssdk.String(request.TransactionID),
		})
		if err != nil {
			return "", err
		}
		return awssdk.ToString(output.TransactionStatus), nil
	}
	output, err := c.RDSData.RollbackTransaction(ctx, &rdsdata.RollbackTransactionInput{
		ResourceArn: awssdk.String(request.ResourceARN), SecretArn: awssdk.String(request.SecretARN),
		TransactionId: awssdk.String(request.TransactionID),
	})
	if err != nil {
		return "", err
	}
	return awssdk.ToString(output.TransactionStatus), nil
}

func dataAPIFieldString(field types.Field) string {
	switch value := field.(type) {
	case *types.FieldMemberIsNull:
		if value.Value {
			return "NULL"
		}
	case *types.FieldMemberStringValue:
		return value.Value
	case *types.FieldMemberLongValue:
		return strconv.FormatInt(value.Value, 10)
	case *types.FieldMemberDoubleValue:
		return strconv.FormatFloat(value.Value, 'g', -1, 64)
	case *types.FieldMemberBooleanValue:
		return strconv.FormatBool(value.Value)
	case *types.FieldMemberBlobValue:
		return base64.StdEncoding.EncodeToString(value.Value)
	case *types.FieldMemberArrayValue:
		encoded, _ := json.Marshal(dataAPIArray(value.Value))
		return string(encoded)
	}
	return ""
}

func dataAPIArray(value types.ArrayValue) any {
	switch array := value.(type) {
	case *types.ArrayValueMemberStringValues:
		return array.Value
	case *types.ArrayValueMemberLongValues:
		return array.Value
	case *types.ArrayValueMemberDoubleValues:
		return array.Value
	case *types.ArrayValueMemberBooleanValues:
		return array.Value
	case *types.ArrayValueMemberArrayValues:
		values := make([]any, len(array.Value))
		for index, nested := range array.Value {
			values[index] = dataAPIArray(nested)
		}
		return values
	default:
		return nil
	}
}
