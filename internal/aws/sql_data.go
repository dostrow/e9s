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
	output, err := c.RDSData.ExecuteStatement(ctx, &rdsdata.ExecuteStatementInput{
		ResourceArn: awssdk.String(request.ResourceARN), SecretArn: awssdk.String(request.SecretARN),
		Database: awssdk.String(request.Database), Sql: awssdk.String(request.SQL), IncludeResultMetadata: true,
	})
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
