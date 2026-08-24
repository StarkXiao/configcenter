package domain

import (
	"encoding/json"
	"testing"
)

func TestCanonicalJSONIsStable(t *testing.T) {
	left := []ConfigItem{
		{Key: "feature.enabled", Type: ValueBoolean, Value: "true"},
		{Key: "database.pool", Type: ValueNumber, Value: "20"},
	}
	right := []ConfigItem{left[1], left[0]}
	leftJSON, leftChecksum, err := CanonicalJSON(left)
	if err != nil {
		t.Fatal(err)
	}
	rightJSON, rightChecksum, err := CanonicalJSON(right)
	if err != nil {
		t.Fatal(err)
	}
	if string(leftJSON) != string(rightJSON) || leftChecksum != rightChecksum {
		t.Fatalf("canonical output differs: %s != %s", leftJSON, rightJSON)
	}
	if !json.Valid(leftJSON) {
		t.Fatalf("invalid JSON: %s", leftJSON)
	}
}

func TestValidateItemsRejectsDuplicateAndInvalidValue(t *testing.T) {
	tests := []struct {
		name  string
		items []ConfigItem
	}{
		{"duplicate", []ConfigItem{{Key: "app.mode", Type: ValueString}, {Key: "app.mode", Type: ValueString}}},
		{"invalid boolean", []ConfigItem{{Key: "app.enabled", Type: ValueBoolean, Value: "yes"}}},
		{"scalar JSON", []ConfigItem{{Key: "app.options", Type: ValueJSON, Value: `"value"`}}},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if err := ValidateItems(test.items); err == nil {
				t.Fatal("expected validation error")
			}
		})
	}
}

func TestCompareMasksSensitiveValues(t *testing.T) {
	current := []ConfigItem{{Key: "database.password", Type: ValueString, Value: "old", Sensitive: true}}
	candidate := []ConfigItem{
		{Key: "database.password", Type: ValueString, Value: "new", Sensitive: true},
		{Key: "feature.enabled", Type: ValueBoolean, Value: "true"},
	}
	diff := Compare(current, candidate, false)
	if len(diff.Added) != 1 || len(diff.Modified) != 1 || len(diff.Deleted) != 0 {
		t.Fatalf("unexpected diff: %+v", diff)
	}
	if diff.Modified[0].Value != "******" {
		t.Fatalf("sensitive value was exposed: %q", diff.Modified[0].Value)
	}
}
