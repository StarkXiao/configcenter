package domain

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"sort"
	"strconv"
	"strings"
)

type ValueType string

const (
	ValueString    ValueType = "string"
	ValueNumber    ValueType = "number"
	ValueBoolean   ValueType = "boolean"
	ValueJSON      ValueType = "json"
	MaxKeyBytes              = 128
	MaxValueBytes            = 64 * 1024
	MaxConfigBytes           = 1024 * 1024
)

type ConfigItem struct {
	Key         string    `json:"key"`
	Value       string    `json:"value"`
	Type        ValueType `json:"type"`
	Description string    `json:"description"`
	Sensitive   bool      `json:"sensitive"`
}

type Draft struct {
	Items          []ConfigItem `json:"items"`
	Revision       int64        `json:"revision"`
	CurrentVersion int64        `json:"current_version"`
}

type Diff struct {
	Added    []ConfigItem `json:"added"`
	Modified []ConfigItem `json:"modified"`
	Deleted  []ConfigItem `json:"deleted"`
}

func ValidateItems(items []ConfigItem) error {
	seen := make(map[string]struct{}, len(items))
	total := 0
	for index := range items {
		item := &items[index]
		item.Key = strings.TrimSpace(item.Key)
		item.Description = strings.TrimSpace(item.Description)
		if err := ValidateItem(*item); err != nil {
			return err
		}
		if _, ok := seen[item.Key]; ok {
			return NewError(CodeInvalid, "duplicate configuration key: "+item.Key)
		}
		seen[item.Key] = struct{}{}
		total += len(item.Key) + len(item.Value) + len(item.Description)
		if total > MaxConfigBytes {
			return NewError(CodeTooLarge, "configuration exceeds one megabyte")
		}
	}
	return nil
}

func ValidateItem(item ConfigItem) error {
	if item.Key == "" || len(item.Key) > MaxKeyBytes {
		return NewError(CodeInvalid, "configuration key must contain 1 to 128 bytes")
	}
	if strings.HasPrefix(item.Key, ".") || strings.HasSuffix(item.Key, ".") || strings.Contains(item.Key, "..") {
		return NewError(CodeInvalid, "configuration key contains an invalid dot sequence: "+item.Key)
	}
	for _, part := range strings.Split(item.Key, ".") {
		if !environmentPattern.MatchString(part) {
			return NewError(CodeInvalid, "configuration key contains an invalid segment: "+item.Key)
		}
	}
	if len(item.Value) > MaxValueBytes {
		return NewError(CodeTooLarge, "configuration value exceeds 64 KB: "+item.Key)
	}
	switch item.Type {
	case ValueString:
		return nil
	case ValueNumber:
		if _, err := strconv.ParseFloat(item.Value, 64); err != nil {
			return NewError(CodeInvalid, "invalid number for key: "+item.Key)
		}
	case ValueBoolean:
		if item.Value != "true" && item.Value != "false" {
			return NewError(CodeInvalid, "invalid boolean for key: "+item.Key)
		}
	case ValueJSON:
		var value any
		decoder := json.NewDecoder(strings.NewReader(item.Value))
		decoder.UseNumber()
		if err := decoder.Decode(&value); err != nil {
			return NewError(CodeInvalid, "invalid JSON for key: "+item.Key)
		}
		switch value.(type) {
		case map[string]any, []any:
		default:
			return NewError(CodeInvalid, "JSON value must be an object or array: "+item.Key)
		}
	default:
		return NewError(CodeInvalid, "unsupported value type for key: "+item.Key)
	}
	return nil
}

func CanonicalJSON(items []ConfigItem) ([]byte, string, error) {
	ordered := append([]ConfigItem(nil), items...)
	sort.Slice(ordered, func(i, j int) bool { return ordered[i].Key < ordered[j].Key })
	result := make(map[string]any, len(ordered))
	for _, item := range ordered {
		value, err := TypedValue(item)
		if err != nil {
			return nil, "", err
		}
		result[item.Key] = value
	}
	content, err := json.Marshal(result)
	if err != nil {
		return nil, "", WrapError(CodeInternal, "encode configuration", err)
	}
	sum := sha256.Sum256(content)
	return content, hex.EncodeToString(sum[:]), nil
}

func TypedValue(item ConfigItem) (any, error) {
	switch item.Type {
	case ValueString:
		return item.Value, nil
	case ValueNumber:
		return json.Number(item.Value), nil
	case ValueBoolean:
		return item.Value == "true", nil
	case ValueJSON:
		var value any
		decoder := json.NewDecoder(bytes.NewBufferString(item.Value))
		decoder.UseNumber()
		if err := decoder.Decode(&value); err != nil {
			return nil, fmt.Errorf("decode %s: %w", item.Key, err)
		}
		return value, nil
	default:
		return nil, NewError(CodeInvalid, "unsupported value type")
	}
}

func Compare(current, candidate []ConfigItem, revealSensitive bool) Diff {
	left := itemMap(current)
	right := itemMap(candidate)
	diff := Diff{Added: []ConfigItem{}, Modified: []ConfigItem{}, Deleted: []ConfigItem{}}
	for key, item := range right {
		old, exists := left[key]
		if !exists {
			diff.Added = append(diff.Added, mask(item, revealSensitive))
		} else if old.Value != item.Value || old.Type != item.Type || old.Sensitive != item.Sensitive {
			diff.Modified = append(diff.Modified, mask(item, revealSensitive))
		}
	}
	for key, item := range left {
		if _, exists := right[key]; !exists {
			diff.Deleted = append(diff.Deleted, mask(item, revealSensitive))
		}
	}
	sortItems(diff.Added)
	sortItems(diff.Modified)
	sortItems(diff.Deleted)
	return diff
}

func itemMap(items []ConfigItem) map[string]ConfigItem {
	result := make(map[string]ConfigItem, len(items))
	for _, item := range items {
		result[item.Key] = item
	}
	return result
}

func sortItems(items []ConfigItem) {
	sort.Slice(items, func(i, j int) bool { return items[i].Key < items[j].Key })
}

func mask(item ConfigItem, reveal bool) ConfigItem {
	if item.Sensitive && !reveal {
		item.Value = "******"
	}
	return item
}
