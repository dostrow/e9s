package aws

import (
	"encoding/json"
	"testing"

	dbtypes "github.com/aws/aws-sdk-go-v2/service/dynamodb/types"
)

func TestDynamoValueToEditableString_MapProducesValidJSON(t *testing.T) {
	got := DynamoValueToEditableString(map[string]interface{}{
		"count": float64(2),
		"name":  "Alice",
	})

	var decoded map[string]interface{}
	if err := json.Unmarshal([]byte(got), &decoded); err != nil {
		t.Fatalf("editable value is not valid JSON: %v\n%s", err, got)
	}
	if decoded["name"] != "Alice" {
		t.Errorf("name = %v, want %q", decoded["name"], "Alice")
	}
}

func TestEditedDynamoValueAttributeValue_PreservesMap(t *testing.T) {
	av, err := editedDynamoValueAttributeValue(
		map[string]interface{}{"name": "Alice"},
		`{"name":"Bob","count":2}`,
	)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	m, ok := av.(*dbtypes.AttributeValueMemberM)
	if !ok {
		t.Fatalf("attribute value = %T, want M", av)
	}
	if name, ok := m.Value["name"].(*dbtypes.AttributeValueMemberS); !ok || name.Value != "Bob" {
		t.Fatalf("name attribute = %#v, want string Bob", m.Value["name"])
	}
	if _, ok := m.Value["count"].(*dbtypes.AttributeValueMemberN); !ok {
		t.Fatalf("count attribute = %T, want N", m.Value["count"])
	}
}

func TestEditedDynamoValueAttributeValue_InvalidMapDoesNotBecomeString(t *testing.T) {
	_, err := editedDynamoValueAttributeValue(
		map[string]interface{}{"name": "Alice"},
		`{"name": Alice}`,
	)
	if err == nil {
		t.Fatal("expected invalid map edit to return an error")
	}
}

func TestEditedDynamoValueAttributeValue_PreservesStringThatLooksTyped(t *testing.T) {
	av, err := editedDynamoValueAttributeValue("old", `{"name":"Bob"}`)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	s, ok := av.(*dbtypes.AttributeValueMemberS)
	if !ok {
		t.Fatalf("attribute value = %T, want S", av)
	}
	if s.Value != `{"name":"Bob"}` {
		t.Errorf("string value = %q, want JSON-looking string", s.Value)
	}
}
