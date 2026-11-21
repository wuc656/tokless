package tools

import "testing"

func TestParseCodegraphIndexStatus(t *testing.T) {
	tests := []struct {
		name      string
		raw       string
		healthy   bool
		needsSync bool
	}{
		{
			name:      "clean healthy index",
			raw:       `{"initialized":true,"pendingChanges":{"added":0,"modified":0,"removed":0},"index":{"state":"complete","pendingRefs":0,"reindexRecommended":false,"filesNeedingReindex":0}}`,
			healthy:   true,
			needsSync: false,
		},
		{
			name:      "healthy index with pending modified files",
			raw:       `{"initialized":true,"pendingChanges":{"added":3,"modified":22,"removed":0},"index":{"state":"complete","pendingRefs":0,"reindexRecommended":false,"filesNeedingReindex":0}}`,
			healthy:   true,
			needsSync: true,
		},
		{
			name:      "healthy index with filesNeedingReindex",
			raw:       `{"initialized":true,"pendingChanges":{"added":0,"modified":0,"removed":0},"index":{"state":"complete","pendingRefs":0,"reindexRecommended":false,"filesNeedingReindex":2}}`,
			healthy:   true,
			needsSync: true,
		},
		{
			name:      "unhealthy index recommending reindex",
			raw:       `{"initialized":true,"pendingChanges":{"added":1,"modified":0,"removed":0},"index":{"state":"complete","pendingRefs":0,"reindexRecommended":true,"filesNeedingReindex":0}}`,
			healthy:   false,
			needsSync: true,
		},
		{
			name:      "invalid json",
			raw:       `not-json`,
			healthy:   false,
			needsSync: false,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			healthy, needsSync := parseCodegraphIndexStatus(tc.raw)
			if healthy != tc.healthy || needsSync != tc.needsSync {
				t.Fatalf("parseCodegraphIndexStatus(%s) = (healthy=%v, needsSync=%v), want (%v, %v)",
					tc.name, healthy, needsSync, tc.healthy, tc.needsSync)
			}
		})
	}
}
