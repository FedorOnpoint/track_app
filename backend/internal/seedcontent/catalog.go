package seedcontent

import (
	"errors"
	"fmt"
	"strings"
)

// ValidateCatalog checks relationships which individual category validation cannot see.
// Partial category imports may refer to topics already stored in the database; this
// check instead applies to a complete catalog snapshot supplied to the editorial gate.
func ValidateCatalog(documents []Document, production bool) error {
	var problems []error
	categories, orders := map[string]bool{}, map[int]bool{}
	topics := map[string]Topic{}
	questions := map[string]string{}
	for _, d := range documents {
		if err := Validate(d); err != nil {
			problems = append(problems, fmt.Errorf("%s: %w", d.Category.Slug, err))
		}
		if categories[d.Category.Slug] {
			problems = append(problems, fmt.Errorf("duplicate category: %s", d.Category.Slug))
		}
		categories[d.Category.Slug] = true
		if orders[d.Category.SortOrder] {
			problems = append(problems, fmt.Errorf("duplicate category sort_order: %d", d.Category.SortOrder))
		}
		orders[d.Category.SortOrder] = true
		for _, t := range d.Topics {
			if _, ok := topics[t.Slug]; ok {
				problems = append(problems, fmt.Errorf("duplicate topic: %s", t.Slug))
			}
			topics[t.Slug] = t
			for _, q := range t.Questions {
				key := strings.ToLower(strings.Join(strings.Fields(q.Question), " "))
				if previous, ok := questions[key]; ok {
					problems = append(problems, fmt.Errorf("duplicate question in %s and %s: %s", previous, t.Slug, q.Question))
				}
				questions[key] = t.Slug
			}
		}
	}
	if production && (len(categories) != 13 || len(topics) < 80 || len(topics) > 120) {
		problems = append(problems, fmt.Errorf("production catalog requires 13 categories and 80–120 topics; got %d and %d", len(categories), len(topics)))
	}
	state := map[string]int{}
	var visit func(string)
	visit = func(slug string) {
		if state[slug] == 2 {
			return
		}
		if state[slug] == 1 {
			problems = append(problems, fmt.Errorf("prerequisite cycle at %s", slug))
			return
		}
		state[slug] = 1
		for _, p := range topics[slug].Prerequisites {
			if _, ok := topics[p]; !ok {
				problems = append(problems, fmt.Errorf("%s: unknown prerequisite %s", slug, p))
				continue
			}
			visit(p)
		}
		state[slug] = 2
	}
	for slug := range topics {
		visit(slug)
	}
	return errors.Join(problems...)
}
