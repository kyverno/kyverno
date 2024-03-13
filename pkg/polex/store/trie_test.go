package store

import (
	"testing"

	kyvernov2beta1 "github.com/kyverno/kyverno/api/kyverno/v2beta1"
	"k8s.io/apimachinery/pkg/types"
)

type InsertRecord struct {
	key string
	uid types.UID
}

type DeleteRecord struct {
	key string
	uid types.UID
}

type SearchRecord struct {
	key          string
	expectedUIDs []types.UID
}

func TestTrie(t *testing.T) {
	tests := []struct {
		name     string
		inserts  []InsertRecord
		deletes  []DeleteRecord
		searches []SearchRecord
	}{
		{
			name:     "Basic Test",
			inserts:  []InsertRecord{{"ad", types.UID("1")}},
			deletes:  []DeleteRecord{},
			searches: []SearchRecord{{"ad", []types.UID{"1"}}},
		},
		{
			name:     "Wildcard Matches zero Character",
			inserts:  []InsertRecord{{"a*d", types.UID("1")}},
			deletes:  []DeleteRecord{},
			searches: []SearchRecord{{"ad", []types.UID{"1"}}},
		},
		{
			name:     "Wildcard Matches one Character",
			inserts:  []InsertRecord{{"a*d", types.UID("1")}},
			deletes:  []DeleteRecord{},
			searches: []SearchRecord{{"abd", []types.UID{"1"}}},
		},
		{
			name:     "Wildcard Matches two Character",
			inserts:  []InsertRecord{{"a*d", types.UID("1")}},
			deletes:  []DeleteRecord{},
			searches: []SearchRecord{{"abcd", []types.UID{"1"}}},
		},
		{
			name:     "Multiple Wildcard Inserts Match",
			inserts:  []InsertRecord{{"a*d", types.UID("1")}, {"*", types.UID("2")}},
			deletes:  []DeleteRecord{},
			searches: []SearchRecord{{"abcd", []types.UID{"1", "2"}}},
		},
		{
			name:     "Delete",
			inserts:  []InsertRecord{{"a*d", types.UID("1")}, {"*", types.UID("2")}},
			deletes:  []DeleteRecord{{"a*d", types.UID("1")}},
			searches: []SearchRecord{{"abcd", []types.UID{"2"}}},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			trie := NewTrie()

			for _, record := range tt.inserts {
				polex := &kyvernov2beta1.PolicyException{}
				polex.UID = record.uid
				trie.Insert(record.key, polex)
			}

			for _, record := range tt.deletes {
				trie.Delete(record.key, record.uid)
			}

			for _, record := range tt.searches {
				results := trie.Search(record.key)
				if len(results) != len(record.expectedUIDs) {
					t.Errorf("Search failed. Expected UIDs: %v, Got: %v", record.expectedUIDs, results)
					continue
				}
				for i, result := range results {
					if result.UID != record.expectedUIDs[i] {
						t.Errorf("Search failed. Expected UID: %v, Got: %v", record.expectedUIDs[i], result.UID)
					}
				}
			}
		})
	}
}
