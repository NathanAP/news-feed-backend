package schemas

// Keyword suggestions power the feed-building UI: as the user assembles a feed, the client asks for
// keywords to add. See GET /v1/feeds/keyword-suggestions.

const (
	// KeywordSuggestionsDefaultLimit is how many suggestions are returned when the client does not
	// ask for a specific count.
	KeywordSuggestionsDefaultLimit = 10
	// KeywordSuggestionsMaxLimit caps the count so a request can never ask the server to rank and
	// return an unbounded list.
	KeywordSuggestionsMaxLimit = 50
)

// Suggestion strategies, echoed back in the response so the client can label the list ("Related to
// your picks" vs "Popular right now") and so the fallback from related to popular is observable
// without reading server logs.
const (
	KeywordStrategyRelated = "related"
	KeywordStrategyPopular = "popular"
)

// KeywordSuggestion is one suggested keyword and how many articles currently carry it (the popularity
// signal the ranking is built on).
type KeywordSuggestion struct {
	Keyword string `json:"keyword"`
	Count   int64  `json:"count"`
}

// KeywordSuggestionsResponse is the endpoint's payload: the strategy that produced the list plus the
// ranked suggestions. Suggestions is always a (possibly empty) array, never null.
type KeywordSuggestionsResponse struct {
	Strategy    string              `json:"strategy"`
	Suggestions []KeywordSuggestion `json:"suggestions"`
}
