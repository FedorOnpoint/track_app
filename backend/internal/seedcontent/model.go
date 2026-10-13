package seedcontent

type Document struct {
	Version  int      `json:"version"`
	Category Category `json:"category"`
	Topics   []Topic  `json:"topics"`
}

type Category struct {
	Slug        string `json:"slug"`
	Title       string `json:"title"`
	Description string `json:"description"`
	SortOrder   int    `json:"sort_order"`
}

type Topic struct {
	Slug             string          `json:"slug"`
	Title            string          `json:"title"`
	Summary          string          `json:"summary"`
	Difficulty       string          `json:"difficulty"`
	SortOrder        int             `json:"sort_order"`
	EstimatedMinutes int             `json:"estimated_minutes"`
	Prerequisites    []string        `json:"prerequisites"`
	Sections         []Section       `json:"sections"`
	Checklist        []ChecklistItem `json:"checklist"`
	Questions        []Question      `json:"questions"`
	Resources        []Resource      `json:"resources"`
	Practice         []Practice      `json:"practice,omitempty"`
}

type Section struct {
	Heading      string `json:"heading"`
	BodyMarkdown string `json:"body_markdown"`
	SortOrder    int    `json:"sort_order"`
}

type ChecklistItem struct {
	StableKey string `json:"stable_key"`
	Text      string `json:"text"`
	SortOrder int    `json:"sort_order"`
}

type Question struct {
	StableKey      string     `json:"stable_key"`
	Question       string     `json:"question"`
	ShortAnswer    string     `json:"short_answer"`
	DetailedAnswer string     `json:"detailed_answer"`
	Pitfalls       string     `json:"pitfalls"`
	SortOrder      int        `json:"sort_order"`
	FollowUps      []FollowUp `json:"follow_ups,omitempty"`
}

type FollowUp struct {
	Question       string `json:"question"`
	ExpectedAnswer string `json:"expected_answer"`
	SortOrder      int    `json:"sort_order"`
}

type Resource struct {
	Title        string `json:"title"`
	URL          string `json:"url"`
	ResourceType string `json:"resource_type"`
	SourceName   string `json:"source_name"`
	SortOrder    int    `json:"sort_order"`
}

type Practice struct {
	Title           string `json:"title"`
	Description     string `json:"description"`
	URL             string `json:"url,omitempty"`
	ExpectedOutcome string `json:"expected_outcome"`
	SortOrder       int    `json:"sort_order"`
}
