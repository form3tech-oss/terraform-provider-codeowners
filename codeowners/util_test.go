package codeowners

import (
	"fmt"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestRulesetComparison(t *testing.T) {
	tests := []struct {
		name     string
		ruleset1 Ruleset
		ruleset2 Ruleset
		match    bool
	}{
		{
			name:     "empty rulesets",
			ruleset1: Ruleset{},
			ruleset2: Ruleset{},
			match:    true,
		},
		{
			name:     "one empty ruleset, one non-empty",
			ruleset1: Ruleset{},
			ruleset2: Ruleset{Rule{Pattern: "*", Usernames: []string{"someone"}}},
			match:    false,
		},
		{
			name:     "identical rulesets",
			ruleset1: Ruleset{Rule{Pattern: "*", Usernames: []string{"someone"}}},
			ruleset2: Ruleset{Rule{Pattern: "*", Usernames: []string{"someone"}}},
			match:    true,
		},
		{
			name:     "different patterns",
			ruleset1: Ruleset{Rule{Pattern: "*.go", Usernames: []string{"someone"}}},
			ruleset2: Ruleset{Rule{Pattern: "*", Usernames: []string{"someone"}}},
			match:    false,
		},
		{
			name:     "different usernames",
			ruleset1: Ruleset{Rule{Pattern: "*", Usernames: []string{"someone"}}},
			ruleset2: Ruleset{Rule{Pattern: "*", Usernames: []string{"someone-else"}}},
			match:    false,
		},
		{
			name: "different usernames, matching patterns + swapped",
			ruleset1: Ruleset{
				Rule{Pattern: "*", Usernames: []string{"someone1"}},
				Rule{Pattern: "*.go", Usernames: []string{"someone2"}},
			},
			ruleset2: Ruleset{
				Rule{Pattern: "*", Usernames: []string{"someone2"}},
				Rule{Pattern: "*.go", Usernames: []string{"someone1"}},
			},
			match: false,
		},
		{
			name: "identical rulesets with multiple rules",
			ruleset1: Ruleset{
				Rule{Pattern: "*", Usernames: []string{"someone"}},
				Rule{Pattern: "*.go", Usernames: []string{"someone"}},
			},
			ruleset2: Ruleset{
				Rule{Pattern: "*", Usernames: []string{"someone"}},
				Rule{Pattern: "*.go", Usernames: []string{"someone"}},
			},
			match: true,
		},
		{
			name: "identical rulesets with multiple rules in differing orders",
			ruleset1: Ruleset{
				Rule{Pattern: "*", Usernames: []string{"someone"}},
				Rule{Pattern: "*.go", Usernames: []string{"someone"}},
			},
			ruleset2: Ruleset{
				Rule{Pattern: "*.go", Usernames: []string{"someone"}},
				Rule{Pattern: "*", Usernames: []string{"someone"}},
			},
			match: true,
		},
		{
			name: "identical rulesets with multiple rules in differing orders",
			ruleset1: Ruleset{
				Rule{Pattern: "*", Usernames: []string{"someone"}},
				Rule{Pattern: "*.go", Usernames: []string{"someone"}},
			},
			ruleset2: Ruleset{
				Rule{Pattern: "*.go", Usernames: []string{"someone"}},
				Rule{Pattern: "*", Usernames: []string{"someone"}},
			},
			match: true,
		},
		{
			name: "identical rulesets with multiple rules + usernames in differing orders",
			ruleset1: Ruleset{
				Rule{Pattern: "*", Usernames: []string{"jim", "bob"}},
				Rule{Pattern: "*.go", Usernames: []string{"someone"}},
			},
			ruleset2: Ruleset{
				Rule{Pattern: "*", Usernames: []string{"bob", "jim"}},
				Rule{Pattern: "*.go", Usernames: []string{"someone"}},
			},
			match: true,
		},
	}

	for i, test := range tests {
		t.Run(fmt.Sprintf("Comparison test #%d: %s", i, test.name), func(t *testing.T) {
			assert.Equal(t, test.match, test.ruleset1.Equal(test.ruleset2))
		})
	}
}

func TestRulesetCompilation(t *testing.T) {
	tests := []Ruleset{
		Ruleset{
			Rule{Pattern: "*", Usernames: []string{"jim"}},
		},
		Ruleset{
			Rule{Pattern: "*", Usernames: []string{"jim"}},
			Rule{Pattern: "*.go", Usernames: []string{"someone"}},
		},
		Ruleset{
			Rule{Pattern: "*", Usernames: []string{"jim", "bob"}},
			Rule{Pattern: "*.go", Usernames: []string{"someone"}},
		},
		Ruleset{
			Rule{Pattern: "*", Usernames: []string{"jim", "bob"}},
			Rule{Pattern: "*.go", Usernames: []string{"someone", "some-user", "somebody-else"}},
		},
		Ruleset{
			Rule{Pattern: "*", Usernames: []string{"jim", "bob"}},
			Rule{Pattern: "*.go", Usernames: []string{"someone", "some-user", "somebody-else"}},
			Rule{Pattern: "*.java", Usernames: []string{"javaguy123", "javadev"}},
		},
	}

	for i, test := range tests {
		t.Run(fmt.Sprintf("Compilation test #%d", i), func(t *testing.T) {
			actual := string(test.Compile())

			for _, rule := range test {
				found := false
				for _, line := range strings.Split(actual, "\n") {
					if strings.HasPrefix(line, rule.Pattern+" ") {
						found = true
						for _, username := range rule.Usernames {
							assert.Contains(t, line, "@"+username, "Line for pattern %s should contain username @%s", rule.Pattern, username)
						}
					}
				}
				assert.True(t, found, "Compiled output should contain rule for pattern %s", rule.Pattern)
			}

		})
	}
}

func TestRulesetParsing(t *testing.T) {
	ruleset := parseRulesFile(`
# this is an example file

# with blank lines

  # and indented comments

* @user1
*.go @user123 @user456
`)

	require.Len(t, ruleset, 2)
	assert.Equal(t, "*", ruleset[0].Pattern)
	assert.Equal(t, []string{"user1"}, ruleset[0].Usernames)
	assert.Equal(t, "*.go", ruleset[1].Pattern)
	assert.Equal(t, []string{"user123", "user456"}, ruleset[1].Usernames)

}
