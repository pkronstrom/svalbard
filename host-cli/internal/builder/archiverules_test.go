package builder

import (
	"testing"

	"github.com/pkronstrom/svalbard/host-cli/internal/catalog"
)

func TestCompileArchiveRulesRejectsInvalidSelectors(t *testing.T) {
	_, err := compileArchiveRules([]catalog.ArchiveRule{{Domain: "example.test", Remove: []string{"["}}})
	if err == nil {
		t.Fatal("compileArchiveRules accepted invalid CSS selector")
	}
}

func TestArchiveRuleForMatchesExactAndWildcardDomains(t *testing.T) {
	rules, err := compileArchiveRules([]catalog.ArchiveRule{{Domain: "example.test"}, {Domain: "*.example.org"}})
	if err != nil {
		t.Fatal(err)
	}
	if archiveRuleFor("example.test", rules) == nil || archiveRuleFor("www.example.org", rules) == nil || archiveRuleFor("example.org", rules) != nil {
		t.Fatal("archive rule domain matching was incorrect")
	}
}
