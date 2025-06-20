// internal/services/structs.go
package services

// ContentAnalysis 内容分析结果
type ContentAnalysis struct {
	Characters []struct {
		Name        string            `json:"name"`
		Description string            `json:"description"`
		Traits      []string          `json:"traits"`
		Relations   map[string]string `json:"relations,omitempty"`
	} `json:"characters"`

	Scenes []struct {
		Name        string   `json:"name"`
		Description string   `json:"description"`
		Atmosphere  string   `json:"atmosphere"`
		Items       []string `json:"items,omitempty"`
	} `json:"scenes"`

	Props []struct {
		Name        string   `json:"name"`
		Description string   `json:"description"`
		Function    string   `json:"function,omitempty"`
		Connections []string `json:"connections,omitempty"`
	} `json:"props"`

	Plotpoints []struct {
		Description string   `json:"description"`
		Importance  string   `json:"importance"`
		Characters  []string `json:"characters,omitempty"`
	} `json:"plotpoints"`

	Themes []string `json:"themes"`
}

// ScenarioIdeas 场景创意生成结果
type ScenarioIdeas struct {
	Ideas []struct {
		Title      string `json:"title"`
		Setting    string `json:"setting"`
		Premise    string `json:"premise"`
		Characters []struct {
			Role        string   `json:"role"`
			Description string   `json:"description"`
			Goals       []string `json:"goals,omitempty"`
		} `json:"characters"`
		Conflicts []string `json:"conflicts"`
		Branches  []struct {
			Description string   `json:"description"`
			Options     []string `json:"options,omitempty"`
		} `json:"branches,omitempty"`
	} `json:"ideas"`
}

// LocationInfo 位置信息
type LocationInfo struct {
	Name        string   `json:"name"`
	Description string   `json:"description"`
	Items       []string `json:"items,omitempty"`
	Connections []string `json:"connections,omitempty"`
}

// LocationAnalysis 位置分析结果
type LocationAnalysis struct {
	SpatialRelations []struct {
		Location1   string `json:"location1"`
		Location2   string `json:"location2"`
		Relation    string `json:"relation"`
		PathDetails string `json:"path_details,omitempty"`
	} `json:"spatial_relations"`

	LocationFunctions map[string]string `json:"location_functions"`

	RecommendedPaths []struct {
		Path           []string `json:"path"`
		Description    string   `json:"description"`
		StoryPotential string   `json:"story_potential"`
	} `json:"recommended_paths"`

	StoryFlowSuggestions []string `json:"story_flow_suggestions"`
}

// CharacterInteraction 角色互动结果
type CharacterInteraction struct {
	Conversation []struct {
		Speaker  string `json:"speaker"`
		Dialogue string `json:"dialogue"`
		Emotion  string `json:"emotion,omitempty"`
		Action   string `json:"action,omitempty"`
	} `json:"conversation"`

	RelationshipDynamics string `json:"relationship_dynamics"`

	PotentialOutcomes []struct {
		Description string `json:"description"`
		Impact      string `json:"impact,omitempty"`
	} `json:"potential_outcomes,omitempty"`
}
