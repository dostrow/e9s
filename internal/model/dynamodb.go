package model

// DynamoTable is the shared table summary and schema used by both frontends.
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
	Name     string
	Type     string
	AttrType string
}

type DynamoItem map[string]any

// DynamoPage is one scan page. NextToken is opaque navigation state owned by
// the shared service and remains free of AWS SDK types.
type DynamoPage struct {
	Items        []DynamoItem
	Count        int
	ScannedCount int
	NextToken    string
}

type DynamoFilter struct {
	Attribute string
	Operator  string
	Value     string
}

type DynamoScanRequest struct {
	Table     string
	Limit     int
	NextToken string
	Filter    *DynamoFilter
}

type DynamoFieldUpdate struct {
	Table         string
	KeyNames      []string
	Item          DynamoItem
	Attribute     string
	OriginalValue any
	NewValue      string
}

// DynamoPutRequest describes a guarded item creation. KeyNames are required so
// the storage layer can reject an existing key instead of silently replacing
// an item through DynamoDB's PutItem semantics.
type DynamoPutRequest struct {
	Table    string
	KeyNames []string
	Item     DynamoItem
}
