package list

import (
	"fmt"
	"strings"
)

const (
	MaxItems = 10_000
	MaxDepth = 3
)

type ItemID string

type SearchTier uint8

const (
	SearchPrimary SearchTier = iota + 1
	SearchSecondary
)

type SearchField struct {
	Tier SearchTier
	Text string
}

type Item struct {
	ID           ItemID
	Primary      string
	Secondary    string
	SearchFields []SearchField
	Children     []Item
}

type Snapshot struct {
	Items        []Item
	ActiveItemID ItemID
	Notice       string
}

func Validate(snapshot Snapshot) error {
	ids := make(map[ItemID]struct{})
	count := 0
	var validateItems func([]Item, int) error
	validateItems = func(items []Item, depth int) error {
		if depth > MaxDepth {
			return fmt.Errorf("item tree exceeds maximum depth %d", MaxDepth)
		}
		for i, item := range items {
			count++
			if count > MaxItems {
				return fmt.Errorf("item tree exceeds maximum count %d", MaxItems)
			}
			if strings.TrimSpace(string(item.ID)) == "" {
				return fmt.Errorf("item at depth %d index %d has an empty ID", depth, i)
			}
			if _, exists := ids[item.ID]; exists {
				return fmt.Errorf("duplicate item ID %q", item.ID)
			}
			ids[item.ID] = struct{}{}
			if strings.TrimSpace(item.Primary) == "" {
				return fmt.Errorf("item %q has empty primary text", item.ID)
			}
			previousTier := SearchPrimary
			for fieldIndex, field := range item.SearchFields {
				if field.Tier != SearchPrimary && field.Tier != SearchSecondary {
					return fmt.Errorf("item %q search field %d has invalid tier %d", item.ID, fieldIndex, field.Tier)
				}
				if fieldIndex > 0 && field.Tier < previousTier {
					return fmt.Errorf("item %q search fields are not ordered by tier", item.ID)
				}
				if strings.TrimSpace(field.Text) == "" {
					return fmt.Errorf("item %q search field %d is empty", item.ID, fieldIndex)
				}
				previousTier = field.Tier
			}
			if len(item.Children) > 0 {
				if err := validateItems(item.Children, depth+1); err != nil {
					return err
				}
			}
		}
		return nil
	}
	if err := validateItems(snapshot.Items, 1); err != nil {
		return err
	}
	if snapshot.ActiveItemID != "" {
		if _, exists := ids[snapshot.ActiveItemID]; !exists {
			return fmt.Errorf("active item ID %q is not present", snapshot.ActiveItemID)
		}
	}
	return nil
}

func IDs(items []Item) map[ItemID]struct{} {
	ids := make(map[ItemID]struct{})
	var collect func([]Item)
	collect = func(items []Item) {
		for _, item := range items {
			ids[item.ID] = struct{}{}
			collect(item.Children)
		}
	}
	collect(items)
	return ids
}
