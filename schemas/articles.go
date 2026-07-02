package schemas

import "time"

const (
	ArticleKeywordsMin = 5
	ArticleKeywordsMax = 20
)

type CreateArticleRequest struct {
	Title       string   `json:"title"`
	Content     string   `json:"content"`
	URLOriginal string   `json:"url_original"`
	Keywords    []string `json:"keywords"`
	SourceID    string   `json:"source_id"`
	// LanguageOriginal is the article's original language (a Language enum code). The manual
	// create/update endpoints require it; the CRON pipeline detects it via lingua-go instead.
	LanguageOriginal string `json:"language_original"`
}

// RawArticleInput is the untreated article data (as it comes out of discovery), used as input to
// the treatment dry-run endpoint.
type RawArticleInput struct {
	Title       string `json:"title"`
	Content     string `json:"content"`
	URLOriginal string `json:"url_original"`
}

type TreatArticleRequest struct {
	Article RawArticleInput `json:"article"`
	// KeywordsMode optionally overrides the keyword-naming mode (local | groq | gemini) for this
	// dry-run only, so different backends can be benchmarked from Bruno without restarting.
	KeywordsMode string `json:"keywords_mode"`
}

// ArticleTreatmentResponse is the result of treating a raw article: cleaned content plus the
// assigned keywords, with the mode used and per-step timings for benchmarking. LanguageOriginal is
// the language detected by lingua-go (empty when detection was not reliable).
type ArticleTreatmentResponse struct {
	Content          string   `json:"content"`
	Keywords         []string `json:"keywords"`
	KeywordsMode     string   `json:"keywords_mode"`
	LanguageOriginal string   `json:"language_original"`
	TreatmentMs      int64    `json:"treatment_ms"`
	KeywordsMs       int64    `json:"keywords_ms"`
}

// JudgeArticleInput is a treated article (as it exists right before judgement in the pipeline:
// cleaned content plus assigned keywords) used as input to the judgement dry-run. It carries no id
// on purpose, so simulations do not depend on an article being persisted first.
type JudgeArticleInput struct {
	Title    string   `json:"title"`
	Content  string   `json:"content"`
	Keywords []string `json:"keywords"`
}

type JudgeArticleRequest struct {
	Article JudgeArticleInput `json:"article"`
	// JudgementMode optionally overrides the judgement mode (local | groq | gemini) for this
	// dry-run only, so different backends can be benchmarked without restarting.
	JudgementMode string `json:"judgement_mode"`
}

// FeedJudgement is the score of one candidate feed against the article.
type FeedJudgement struct {
	FeedID   string `json:"feed_id"`
	FeedName string `json:"feed_name"`
	Score    int    `json:"score"`
	Passed   bool   `json:"passed"`
}

// ArticleJudgementResponse is the result of the judgement dry-run: the candidate feeds (layer 1)
// each scored by the AI (layer 2), with the mode/threshold used and the total elapsed time. It
// stops at the penultimate step — no articles_feeds association is written.
type ArticleJudgementResponse struct {
	JudgementMode  string          `json:"judgement_mode"`
	Threshold      int             `json:"threshold"`
	CandidateCount int             `json:"candidate_count"`
	Judgements     []FeedJudgement `json:"judgements"`
	JudgementMs    int64           `json:"judgement_ms"`
}

// UpdateArticleRequest intentionally omits source_id: the article's source is immutable.
type UpdateArticleRequest struct {
	Title       string   `json:"title"`
	Content     string   `json:"content"`
	URLOriginal string   `json:"url_original"`
	Keywords    []string `json:"keywords"`
	// LanguageOriginal is required, like on create — the article's original language.
	LanguageOriginal string `json:"language_original"`
}

type ArticleResponse struct {
	ID          string   `json:"id"`
	Status      bool     `json:"status"`
	Title       string   `json:"title"`
	Content     string   `json:"content"`
	URLOriginal string   `json:"url_original"`
	Keywords    []string `json:"keywords"`
	SourceID    string   `json:"source_id"`
	// LanguageOriginal is the detected/declared original language; null when unknown.
	LanguageOriginal *string    `json:"language_original"`
	CreatedAt        time.Time  `json:"created_at"`
	ModifiedAt       *time.Time `json:"modified_at,omitempty"`
	// IsRead is null when the article is not in any of the requesting user's feeds,
	// false when it is in at least one feed and unread, true when all are read.
	IsRead *bool `json:"is_read"`
}

// ArticleTranslationResponse is the result of the translation endpoint: the title, content and
// keywords translated into the target language for display. Nothing is persisted (read-only).
// LanguageOriginal echoes the article's original language so the client can offer an
// original↔translated toggle.
type ArticleTranslationResponse struct {
	Title            string   `json:"title"`
	Content          string   `json:"content"`
	Keywords         []string `json:"keywords"`
	Language         string   `json:"language"`
	LanguageOriginal string   `json:"language_original"`
}
