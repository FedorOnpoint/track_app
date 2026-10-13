package seedcontent

import (
	"strings"
	"testing"
)

func TestValidateCatalogRejectsCrossCategoryDefects(t *testing.T) {
	base := func() []Document {
		a, b := validDocument(), validDocument()
		b.Category.Slug = "other"
		b.Category.SortOrder = 2
		b.Topics[0].Slug = "other-topic"
		for i := range b.Topics[0].Questions {
			b.Topics[0].Questions[i].Question = "Other " + b.Topics[0].Questions[i].Question
		}
		return []Document{a, b}
	}
	for _, tc := range []struct {
		name, want string
		change     func([]Document)
	}{
		{"unknown prerequisite", "unknown prerequisite", func(d []Document) { d[1].Topics[0].Prerequisites = []string{"missing"} }},
		{"cycle", "cycle", func(d []Document) {
			d[0].Topics[0].Prerequisites = []string{"other-topic"}
			d[1].Topics[0].Prerequisites = []string{"interfaces"}
		}},
		{"duplicate question", "duplicate question", func(d []Document) {
			d[1].Topics[0].Questions[0].Question = "  " + strings.ToUpper(d[0].Topics[0].Questions[0].Question) + "  "
		}},
		{"duplicate topic", "duplicate topic", func(d []Document) { d[1].Topics[0].Slug = "interfaces" }},
		{"duplicate category", "duplicate category", func(d []Document) { d[1].Category.Slug = d[0].Category.Slug }},
		{"category order", "category sort_order", func(d []Document) { d[1].Category.SortOrder = d[0].Category.SortOrder }},
	} {
		t.Run(tc.name, func(t *testing.T) {
			d := base()
			tc.change(d)
			err := ValidateCatalog(d, false)
			if err == nil || !strings.Contains(err.Error(), tc.want) {
				t.Fatalf("error=%v want %s", err, tc.want)
			}
		})
	}
	d := base()
	d[1].Topics[0].Prerequisites = []string{"interfaces"}
	if err := ValidateCatalog(d, false); err != nil {
		t.Fatal(err)
	}
	if err := ValidateCatalog(d, true); err == nil {
		t.Fatal("accepted incomplete production catalog")
	}
}
