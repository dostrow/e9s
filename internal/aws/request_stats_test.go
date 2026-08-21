package aws

import "testing"

func TestEstimatedRequestCostOnlyClaimsKnownDirectCharges(t *testing.T) {
	if got := estimatedRequestCost("Cost Explorer", "GetCostAndUsage"); got != 0.01 {
		t.Fatalf("Cost Explorer request estimate = %v, want 0.01", got)
	}
	if got := estimatedRequestCost("Secrets Manager", "GetSecretValue"); got != 0.000005 {
		t.Fatalf("Secrets Manager request estimate = %v, want 0.000005", got)
	}
	if got := estimatedRequestCost("S3", "ListObjectsV2"); got != 0 {
		t.Fatalf("region-dependent S3 request must not receive a misleading estimate: %v", got)
	}
}
