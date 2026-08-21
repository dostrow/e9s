package ui

import "testing"

func TestParseSQSPollOptions(t *testing.T) {
	maxMessages, waitSeconds, err := parseSQSPollOptions("7, 12")
	if err != nil || maxMessages != 7 || waitSeconds != 12 {
		t.Fatalf("parseSQSPollOptions() = %d, %d, %v", maxMessages, waitSeconds, err)
	}
	for _, value := range []string{"", "10", "0,10", "11,10", "10,-1", "10,21", "ten,10"} {
		if _, _, err := parseSQSPollOptions(value); err == nil {
			t.Errorf("parseSQSPollOptions(%q) succeeded", value)
		}
	}
}
