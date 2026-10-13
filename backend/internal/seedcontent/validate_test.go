package seedcontent

import (
	"bytes"
	"encoding/json"
	"strings"
	"testing"
)

func TestDecodeAndValidateCategory(t *testing.T) {
	data, err := json.Marshal(validDocument())
	if err != nil {
		t.Fatal(err)
	}
	document, err := Decode(bytes.NewReader(data))
	if err != nil {
		t.Fatal(err)
	}
	if err := Validate(document); err != nil {
		t.Fatal(err)
	}
}

func TestDecodeRejectsUnknownFieldsAndTrailingJSON(t *testing.T) {
	for _, input := range []string{`{"version":1,"unknown":true}`, `{} {}`, `null`} {
		if _, err := Decode(strings.NewReader(input)); err == nil {
			t.Fatalf("accepted %s", input)
		}
	}
}

func TestValidateRejectsIncompleteAndUnstableContent(t *testing.T) {
	cases := []struct {
		name   string
		change func(*Document)
		field  string
	}{
		{"version", func(d *Document) { d.Version = 0 }, "version"},
		{"category", func(d *Document) { d.Category.Slug = "Go Core" }, "category.slug"},
		{"empty topics", func(d *Document) { d.Topics = nil }, "topics"},
		{"duplicate slug", func(d *Document) { d.Topics = append(d.Topics, d.Topics[0]) }, "slug"},
		{"summary", func(d *Document) { d.Topics[0].Summary = " " }, "summary"},
		{"difficulty", func(d *Document) { d.Topics[0].Difficulty = "expert" }, "difficulty"},
		{"prerequisites", func(d *Document) { d.Topics[0].Prerequisites = nil }, "prerequisites"},
		{"theory", func(d *Document) { d.Topics[0].Sections = nil }, "sections"},
		{"checklist count", func(d *Document) { d.Topics[0].Checklist = d.Topics[0].Checklist[:2] }, "checklist"},
		{"duplicate key", func(d *Document) { d.Topics[0].Checklist[1].StableKey = "one" }, "stable_key"},
		{"question answer", func(d *Document) { d.Topics[0].Questions[0].DetailedAnswer = "" }, "detailed_answer"},
		{"follow-up answer", func(d *Document) { d.Topics[0].Questions[0].FollowUps[0].ExpectedAnswer = "" }, "expected_answer"},
		{"resource count", func(d *Document) { d.Topics[0].Resources = nil }, "resources"},
		{"resource URL", func(d *Document) { d.Topics[0].Resources[0].URL = "javascript:alert(1)" }, "url"},
		{"resource credentials", func(d *Document) { d.Topics[0].Resources[0].URL = "https://user:password@go.dev/" }, "url"},
		{"resource type", func(d *Document) { d.Topics[0].Resources[0].ResourceType = "invalid" }, "resource_type"},
		{"duplicate order", func(d *Document) { d.Topics[0].Checklist[1].SortOrder = 1 }, "sort_order"},
		{"order overflow", func(d *Document) { d.Topics[0].Checklist[1].SortOrder = 2147483647 }, "sort_order"},
		{"practice outcome", func(d *Document) {
			d.Topics[0].Practice = []Practice{{Title: "Pool", Description: "Write workers", SortOrder: 1}}
		}, "expected_outcome"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			d := validDocument()
			tc.change(&d)
			if err := Validate(d); err == nil || !strings.Contains(err.Error(), tc.field) {
				t.Fatalf("error=%v, want field %s", err, tc.field)
			}
		})
	}
}

func validDocument() Document {
	return Document{Version: 1, Category: Category{Slug: "go-core", Title: "Go Core", Description: "Язык Go", SortOrder: 10}, Topics: []Topic{{
		Slug: "interfaces", Title: "Интерфейсы", Summary: "Неявная реализация", Difficulty: "intermediate", SortOrder: 10, EstimatedMinutes: 25, Prerequisites: []string{},
		Sections:  []Section{{Heading: "Методы", BodyMarkdown: "Набор методов задаёт контракт.", SortOrder: 1}},
		Checklist: []ChecklistItem{{StableKey: "one", Text: "Набор методов", SortOrder: 1}, {StableKey: "two", Text: "Nil-интерфейс", SortOrder: 2}, {StableKey: "three", Text: "Границы", SortOrder: 3}},
		Questions: []Question{{StableKey: "one", Question: "Что такое интерфейс?", ShortAnswer: "Контракт", DetailedAnswer: "Контракт методов", Pitfalls: "Не путать с классом", SortOrder: 1, FollowUps: []FollowUp{{Question: "Когда nil?", ExpectedAnswer: "Нет типа и значения", SortOrder: 1}}}, {StableKey: "two", Question: "Что входит в method set?", ShortAnswer: "Методы", DetailedAnswer: "Зависит от receiver", Pitfalls: "Указатель и значение различаются", SortOrder: 2}, {StableKey: "three", Question: "Где определять?", ShortAnswer: "У потребителя", DetailedAnswer: "По нужным операциям", Pitfalls: "Не копировать весь API", SortOrder: 3}},
		Resources: []Resource{{Title: "Go specification", URL: "https://go.dev/ref/spec", ResourceType: "documentation", SourceName: "Go", SortOrder: 1}},
	}}}
}
