package plasmamcp

import (
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/BrobridgeOrg/plasma-plugin/internal/pcontext"
)

// annotate is the first line of every workspace-scoped answer.
//
// The selected workspace is state held outside the conversation, so an answer
// that does not name it lets a stale selection pass for a fresh one. Printing
// it turns that mistake into something the reader can see.
func annotate(st pcontext.State) string {
	name := st.WorkspaceName
	if name == "" {
		name = "(unnamed)"
	}
	profile := st.Profile
	if profile == "" {
		profile = pcontext.DefaultProfile
	}
	return fmt.Sprintf("[workspace=%s name=%s profile=%s]", st.WorkspaceID, name, profile)
}

// renderTable prints a columnar result as fixed-width columns.
func renderTable(columns []string, rows [][]any) string {
	if len(columns) == 0 {
		return "(no columns)"
	}
	widths := make([]int, len(columns))
	for i, c := range columns {
		widths[i] = len(c)
	}
	cells := make([][]string, len(rows))
	for r, row := range rows {
		cells[r] = make([]string, len(columns))
		for i := range columns {
			var v any
			if i < len(row) {
				v = row[i]
			}
			s := formatCell(v)
			cells[r][i] = s
			if len(s) > widths[i] {
				widths[i] = len(s)
			}
		}
	}

	var b strings.Builder
	writeRow := func(values []string) {
		for i, v := range values {
			if i > 0 {
				b.WriteString("  ")
			}
			b.WriteString(v)
			if pad := widths[i] - len(v); pad > 0 && i < len(values)-1 {
				b.WriteString(strings.Repeat(" ", pad))
			}
		}
		b.WriteString("\n")
	}
	writeRow(columns)
	sep := make([]string, len(columns))
	for i := range columns {
		sep[i] = strings.Repeat("-", widths[i])
	}
	writeRow(sep)
	for _, row := range cells {
		writeRow(row)
	}
	return strings.TrimRight(b.String(), "\n")
}

func formatCell(v any) string {
	switch t := v.(type) {
	case nil:
		return "NULL"
	case string:
		return t
	case bool:
		return strconv.FormatBool(t)
	case float64:
		// JSON numbers arrive as float64; print integers without a ".0".
		if t == float64(int64(t)) {
			return strconv.FormatInt(int64(t), 10)
		}
		return strconv.FormatFloat(t, 'f', -1, 64)
	default:
		return fmt.Sprint(v)
	}
}

// parseExpiry accepts a Go duration plus a day unit ("30d"), because API
// lifetimes are discussed in days far more often than in hours.
func parseExpiry(in string) (time.Duration, error) {
	s := strings.TrimSpace(strings.ToLower(in))
	if s == "" {
		return 0, fmt.Errorf("empty duration")
	}
	if rest, ok := strings.CutSuffix(s, "d"); ok {
		days, err := strconv.ParseFloat(rest, 64)
		if err != nil {
			return 0, fmt.Errorf("%q is not a duration: expected forms are 30d, 12h, 90m", in)
		}
		return time.Duration(days * 24 * float64(time.Hour)), nil
	}
	d, err := time.ParseDuration(s)
	if err != nil {
		return 0, fmt.Errorf("%q is not a duration: expected forms are 30d, 12h, 90m", in)
	}
	return d, nil
}
