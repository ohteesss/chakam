package conversation

import (
	_ "embed"
	"fmt"
	"time"
)

//go:embed system_prompt.md
var systemPromptMD string

// buildSystemPrompt returns the static prompt with today's date appended, so the
// model can resolve relative deadlines ("by Friday") into absolute calendar dates.
func buildSystemPrompt() string {
	today := time.Now().Format("Monday, 2 January 2006")
	return fmt.Sprintf("%s\n\n---\n\n## Current Date\n\nToday is **%s**. Use this to convert any relative dates the user mentions (\"by Friday\", \"end of month\", \"next week\") into absolute calendar dates in the description.", systemPromptMD, today)
}
