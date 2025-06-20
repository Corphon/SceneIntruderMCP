// internal/models/event.go
package models

type Event struct {
	ID          string    `json:"id"`
	SceneID     string    `json:"scene_id"`
	Name        string    `json:"name"`
	Description string    `json:"description"`
	Triggers    []Trigger `json:"triggers"`
	Actions     []Action  `json:"actions"`
	IsCompleted bool      `json:"is_completed"`
}

type Trigger struct {
	Type      string            `json:"type"` // dialog, item, time, etc.
	Condition map[string]string `json:"condition"`
}

type Action struct {
	Type   string         `json:"type"` // scene_change, character_state, item_give, etc.
	Params map[string]any `json:"params"`
}
