package codeowners

import (
	"fmt"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestRulesetCompilation(t *testing.T) {
	tests := []Ruleset{
		{
			Rule{Pattern: "*", Usernames: []string{"jim"}},
		},
		{
			Rule{Pattern: "*", Usernames: []string{"jim"}},
			Rule{Pattern: "*.go", Usernames: []string{"someone"}},
		},
		{
			Rule{Pattern: "*", Usernames: []string{"jim", "bob"}},
			Rule{Pattern: "*.go", Usernames: []string{"someone"}},
		},
		{
			Rule{Pattern: "*", Usernames: []string{"jim", "bob"}},
			Rule{Pattern: "*.go", Usernames: []string{"someone", "some-user", "somebody-else"}},
		},
		{
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
