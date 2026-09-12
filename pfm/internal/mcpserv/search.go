package mcpserv

import (
	"context"
	"fmt"

	"hostops/pfm/internal/chat"
	pfmengine "hostops/pfm/internal/engine"
)

// find is chat_find: chat.Find under the tool's candidate limit (default 10,
// maximum 50), projected onto the wire candidate. SelfID names the asking
// session the search left out, so an empty answer is never mistaken for one
// that looked everywhere.
func (current *backend) find(ctx context.Context, input FindInput) (FindOutput, error) {
	if current.chat == nil {
		return FindOutput{}, fmt.Errorf("chat_find verb is not configured")
	}
	limit := input.Limit
	if limit == 0 {
		limit = 10
	}
	if limit < 1 || limit > 50 {
		return FindOutput{}, fmt.Errorf("limit must be between 1 and 50")
	}
	matches, err := current.chat.Find(ctx, chat.FindRequest{
		Excerpt: input.Excerpt, IncludeSelf: input.IncludeSelf,
	})
	if err != nil {
		return FindOutput{}, fmt.Errorf("chat_find: %w", err)
	}
	if len(matches) > limit {
		matches = matches[:limit]
	}
	candidates := make([]FindCandidate, 0, len(matches))
	for _, match := range matches {
		candidates = append(candidates, FindCandidate{
			ID: match.ID, Path: match.Path, Engine: string(pfmengine.Claude),
			Date: match.Last, Hits: match.Hits, Confirmed: true,
		})
	}
	output := FindOutput{
		Candidates: candidates, Count: len(candidates),
		Needles: chat.ExcerptNeedles(input.Excerpt),
	}
	if !input.IncludeSelf {
		output.SelfID = chat.AskingSession()
	}
	return output, nil
}
