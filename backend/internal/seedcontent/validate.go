package seedcontent

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/url"
	"regexp"
	"strings"
	"unicode/utf8"
)

var stableKey = regexp.MustCompile(`^[a-z0-9]+(?:-[a-z0-9]+)*$`)
var errInvalidURL = errors.New("must be an absolute HTTPS URL without credentials")

func Decode(reader io.Reader) (Document, error) {
	limited := &io.LimitedReader{R: reader, N: (8 << 20) + 1}
	decoder := json.NewDecoder(limited)
	decoder.DisallowUnknownFields()
	var document *Document
	if err := decoder.Decode(&document); err != nil {
		return Document{}, fmt.Errorf("decode seed: %w", err)
	}
	if document == nil {
		return Document{}, errors.New("seed must be a JSON object")
	}
	var extra any
	if err := decoder.Decode(&extra); err != io.EOF {
		return Document{}, errors.New("seed must contain exactly one JSON object")
	}
	if limited.N <= 0 {
		return Document{}, errors.New("seed exceeds 8 MiB")
	}
	return *document, nil
}

func Validate(document Document) error {
	v := validator{}
	v.check(document.Version > 0, "version", "must be positive")
	v.key(document.Category.Slug, "category.slug", nil)
	v.text(document.Category.Title, "category.title", 120)
	v.text(document.Category.Description, "category.description", 0)
	v.order(document.Category.SortOrder, "category.sort_order", nil)
	v.count(len(document.Topics), 1, 120, "topics")
	slugs, orders := map[string]bool{}, map[int]bool{}
	for i, topic := range document.Topics {
		path := fmt.Sprintf("topics[%d]", i)
		v.key(topic.Slug, path+".slug", slugs)
		v.order(topic.SortOrder, path+".sort_order", orders)
		v.text(topic.Title, path+".title", 180)
		v.text(topic.Summary, path+".summary", 0)
		v.check(topic.Difficulty == "basic" || topic.Difficulty == "intermediate" || topic.Difficulty == "advanced", path+".difficulty", "invalid difficulty")
		v.check(topic.EstimatedMinutes > 0, path+".estimated_minutes", "must be positive")
		v.check(topic.Prerequisites != nil, path+".prerequisites", "must be an explicit array (empty is allowed)")
		prerequisites := map[string]bool{}
		for j, prerequisite := range topic.Prerequisites {
			field := fmt.Sprintf("%s.prerequisites[%d]", path, j)
			v.key(prerequisite, field, prerequisites)
			v.check(prerequisite != topic.Slug, field, "must not reference itself")
		}
		v.count(len(topic.Sections), 1, 100, path+".sections")
		sectionOrders := map[int]bool{}
		for j, section := range topic.Sections {
			field := fmt.Sprintf("%s.sections[%d]", path, j)
			v.text(section.Heading, field+".heading", 0)
			v.text(section.BodyMarkdown, field+".body_markdown", 0)
			v.order(section.SortOrder, field+".sort_order", sectionOrders)
		}
		v.count(len(topic.Checklist), 3, 7, path+".checklist")
		checklistKeys, checklistOrders := map[string]bool{}, map[int]bool{}
		for j, item := range topic.Checklist {
			field := fmt.Sprintf("%s.checklist[%d]", path, j)
			v.key(item.StableKey, field+".stable_key", checklistKeys)
			v.text(item.Text, field+".text", 0)
			v.order(item.SortOrder, field+".sort_order", checklistOrders)
		}
		v.count(len(topic.Questions), 3, 8, path+".questions")
		questionKeys, questionOrders, questionTexts := map[string]bool{}, map[int]bool{}, map[string]bool{}
		for j, question := range topic.Questions {
			field := fmt.Sprintf("%s.questions[%d]", path, j)
			v.key(question.StableKey, field+".stable_key", questionKeys)
			v.order(question.SortOrder, field+".sort_order", questionOrders)
			v.text(question.Question, field+".question", 0)
			text := strings.ToLower(strings.Join(strings.Fields(question.Question), " "))
			v.check(!questionTexts[text], field+".question", "duplicate question")
			questionTexts[text] = true
			v.text(question.ShortAnswer, field+".short_answer", 0)
			v.text(question.DetailedAnswer, field+".detailed_answer", 0)
			v.text(question.Pitfalls, field+".pitfalls", 0)
			followUpOrders := map[int]bool{}
			for k, followUp := range question.FollowUps {
				followPath := fmt.Sprintf("%s.follow_ups[%d]", field, k)
				v.text(followUp.Question, followPath+".question", 0)
				v.text(followUp.ExpectedAnswer, followPath+".expected_answer", 0)
				v.order(followUp.SortOrder, followPath+".sort_order", followUpOrders)
			}
		}
		v.count(len(topic.Resources), 1, 4, path+".resources")
		resourceOrders := map[int]bool{}
		for j, resource := range topic.Resources {
			field := fmt.Sprintf("%s.resources[%d]", path, j)
			v.text(resource.Title, field+".title", 0)
			v.text(resource.SourceName, field+".source_name", 0)
			v.order(resource.SortOrder, field+".sort_order", resourceOrders)
			v.check(validURL(resource.URL) == nil, field+".url", errInvalidURL.Error())
			switch resource.ResourceType {
			case "documentation", "article", "video", "book", "course", "tool":
			default:
				v.check(false, field+".resource_type", "invalid resource type")
			}
		}
		practiceOrders := map[int]bool{}
		for j, practice := range topic.Practice {
			field := fmt.Sprintf("%s.practice[%d]", path, j)
			v.text(practice.Title, field+".title", 0)
			v.text(practice.Description, field+".description", 0)
			v.text(practice.ExpectedOutcome, field+".expected_outcome", 0)
			v.order(practice.SortOrder, field+".sort_order", practiceOrders)
			if practice.URL != "" {
				v.check(validURL(practice.URL) == nil, field+".url", errInvalidURL.Error())
			}
		}
	}
	return errors.Join(v.errors...)
}

type validator struct{ errors []error }

func (v *validator) check(ok bool, path, message string) {
	if !ok {
		v.errors = append(v.errors, fmt.Errorf("%s: %s", path, message))
	}
}
func (v *validator) text(value, path string, max int) {
	v.check(strings.TrimSpace(value) != "", path, "must not be blank")
	if max > 0 {
		v.check(utf8.RuneCountInString(value) <= max, path, fmt.Sprintf("must contain at most %d characters", max))
	}
}
func (v *validator) count(count, min, max int, path string) {
	v.check(count >= min && count <= max, path, fmt.Sprintf("must contain %d–%d items", min, max))
}
func (v *validator) key(value, path string, seen map[string]bool) {
	v.check(stableKey.MatchString(value), path, "must be a stable lowercase hyphenated key")
	if seen != nil {
		v.check(!seen[value], path, "duplicate key")
		seen[value] = true
	}
}
func (v *validator) order(value int, path string, seen map[int]bool) {
	v.check(value > 0 && value <= 1_000_000, path, "must be between 1 and 1000000")
	if seen != nil {
		v.check(!seen[value], path, "duplicate sort_order")
		seen[value] = true
	}
}
func validURL(address string) error {
	parsed, err := url.Parse(address)
	if err != nil || parsed.Scheme != "https" || parsed.Hostname() == "" || parsed.User != nil || parsed.Opaque != "" {
		return errInvalidURL
	}
	return nil
}
