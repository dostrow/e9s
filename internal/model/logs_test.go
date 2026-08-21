package model

import "testing"

func TestCenterLogEntriesKeepsExactAnchorCentered(t *testing.T) {
	entries := make([]LogEntry, 0, 5000)
	for i := 0; i < 5000; i++ {
		if i == 2500 {
			continue
		}
		entries = append(entries, LogEntry{ID: string(rune(i + 1)), Timestamp: int64(i), Message: "event"})
	}
	anchor := LogEntry{ID: "selected", Timestamp: 2500, Message: "selected event"}

	got := CenterLogEntries(entries, anchor, 2000)
	if len(got) != 2000 {
		t.Fatalf("len = %d, want 2000", len(got))
	}
	if got[1000].Key() != anchor.Key() {
		t.Fatalf("anchor index is not centered: entry[1000] = %#v", got[1000])
	}
	if got[0].Timestamp != 1500 || got[len(got)-1].Timestamp != 3499 {
		t.Fatalf("window = %d–%d, want 1500–3499", got[0].Timestamp, got[len(got)-1].Timestamp)
	}
}

func TestCenterLogEntriesDoesNotDuplicateExistingAnchor(t *testing.T) {
	anchor := LogEntry{ID: "selected", Timestamp: 2, Message: "selected event"}
	got := CenterLogEntries([]LogEntry{
		{ID: "one", Timestamp: 1}, anchor, {ID: "three", Timestamp: 3},
	}, anchor, 10)
	if len(got) != 3 {
		t.Fatalf("len = %d, want 3", len(got))
	}
}
