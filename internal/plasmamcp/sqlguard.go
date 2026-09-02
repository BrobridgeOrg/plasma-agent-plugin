package plasmamcp

import (
	"fmt"
	"strings"
)

// readOnlyStarts are the two statement heads that can only read.
var readOnlyStarts = []string{"select", "with"}

// guardReadOnlySQL refuses anything that is not a single read.
//
// Plasma enforces the same rule server-side, so this is not the last line of
// defence — it exists to fail before the request, and to make the permission
// prompt show exactly the one statement that would run. A prompt the operator
// approves must not be able to carry a second, hidden statement.
func guardReadOnlySQL(sql string) error {
	trimmed := strings.TrimSpace(sql)
	if trimmed == "" {
		return fmt.Errorf("sql is empty")
	}
	// A trailing semicolon still describes one statement; anything after it
	// is a second one.
	body := strings.TrimSpace(strings.TrimSuffix(trimmed, ";"))
	if strings.Contains(stripLiterals(body), ";") {
		return fmt.Errorf("only one statement per call: remove everything after the first ';'")
	}
	lower := strings.ToLower(body)
	for _, start := range readOnlyStarts {
		if strings.HasPrefix(lower, start+" ") || strings.HasPrefix(lower, start+"\n") ||
			strings.HasPrefix(lower, start+"\t") || strings.HasPrefix(lower, start+"(") {
			return nil
		}
	}
	head := body
	if idx := strings.IndexAny(head, " \t\n("); idx > 0 {
		head = head[:idx]
	}
	return fmt.Errorf("only SELECT and WITH are accepted here, got %q: "+
		"this path exists to read data, and Plasma rejects everything else anyway", head)
}

// stripLiterals blanks out quoted text so a semicolon inside a string
// literal or identifier is not mistaken for a statement separator.
func stripLiterals(sql string) string {
	var b strings.Builder
	b.Grow(len(sql))
	var quote rune
	for _, r := range sql {
		switch {
		case quote != 0:
			if r == quote {
				quote = 0
			}
			b.WriteRune(' ')
		case r == '\'' || r == '"' || r == '`':
			quote = r
			b.WriteRune(' ')
		default:
			b.WriteRune(r)
		}
	}
	return b.String()
}
