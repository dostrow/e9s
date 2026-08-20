package sqlworkbench

import (
	"encoding/csv"
	"io"

	"github.com/dostrow/e9s/internal/model"
)

func WriteCSV(writer io.Writer, result model.SQLQueryResult) error {
	output := csv.NewWriter(writer)
	if len(result.Columns) > 0 {
		if err := output.Write(result.Columns); err != nil {
			return err
		}
	}
	for _, row := range result.Rows {
		if err := output.Write(row); err != nil {
			return err
		}
	}
	output.Flush()
	return output.Error()
}
