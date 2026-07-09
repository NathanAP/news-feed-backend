package ai

import "context"

// NewPassthroughTreater returns a Treater that hands the article content back unchanged, skipping
// the LLM review step. It is wired when TREATMENT_AI_ACTIVE=false: the original RSS content flows
// forward as-is, without any AI call (and without consuming quota). The downstream sanitize step
// still runs over this output, so the stored HTML whitelist (bluemonday) is enforced exactly as
// when the LLM is active — skipping the AI never lets unsafe markup through. Only the content is
// returned (the treated body); the title is passed through the pipeline separately, so it is
// ignored here, mirroring what the real treaters return.
func NewPassthroughTreater() Treater {
	return passthroughTreater{}
}

type passthroughTreater struct{}

func (passthroughTreater) Treat(_ context.Context, _ string, content string) (string, error) {
	return content, nil
}
