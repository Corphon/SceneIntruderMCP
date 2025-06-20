// internal/models/item.go
package models

import "time"

type Item struct {
	ID          string            `json:"id"`
	SceneID     string            `json:"scene_id"`
	Name        string            `json:"name"`
	Description string            `json:"description"`
	Location    string            `json:"location,omitempty"`
	ImageURL    string            `json:"image_url"`
	Type        string            `json:"type"` // key, weapon, document, etc.
	Properties  map[string]any    `json:"properties"`
	UsableWith  []string          `json:"usable_with"` // character_ids or other_item_ids
	IsOwned     bool              `json:"is_owned"`
	Source      ContentSourceType `json:"source"`
	FoundAt     time.Time         `json:"found_at,omitempty"`
	CreatedAt   time.Time         `json:"created_at"`
	LastUpdated time.Time         `json:"last_updated"`
}

type ItemInteraction struct {
	ID          string `json:"id"`
	ItemID      string `json:"item_id"`
	TargetID    string `json:"target_id"`   // character_id or another item_id
	TargetType  string `json:"target_type"` // character or item
	Description string `json:"description"`
	Effect      string `json:"effect"`
}
