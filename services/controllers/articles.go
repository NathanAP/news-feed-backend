package controllers

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/nathanap/news-feed-backend/schemas"
	"github.com/nathanap/news-feed-backend/services/utctime"
	db "github.com/nathanap/news-feed-backend/sqlc"
)

var (
	ErrArticleNotFound      = errors.New("article not found")
	ErrArticleAlreadyExists = errors.New("article with this original URL already exists")
	ErrArticleSourceInvalid = errors.New("source not found or inactive")
)

type ArticleController struct{}

func NewArticleController() *ArticleController {
	return &ArticleController{}
}

func (c *ArticleController) Create(ctx context.Context, q db.Querier, title, content, urlOriginal, sourceID string, keywords []string, languageOriginal *string) (db.Article, error) {
	// The article must reference an existing, active source. FindSourceByID already filters
	// status = TRUE AND removed_at IS NULL, so a soft-deleted source resolves to not-found.
	if _, err := q.FindSourceByID(ctx, sourceID); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return db.Article{}, ErrArticleSourceInvalid
		}
		return db.Article{}, fmt.Errorf("failed to validate source: %w", err)
	}

	id, err := uuid.NewV7()
	if err != nil {
		return db.Article{}, fmt.Errorf("failed to generate article ID: %w", err)
	}

	encodedKeywords, err := encodeKeywords(keywords)
	if err != nil {
		return db.Article{}, err
	}

	article, err := q.CreateArticle(ctx, db.CreateArticleParams{
		ID:               id.String(),
		Title:            title,
		Content:          content,
		UrlOriginal:      urlOriginal,
		Keywords:         encodedKeywords,
		SourceID:         sourceID,
		LanguageOriginal: nullString(languageOriginal),
	})
	if err != nil {
		if isUniqueViolation(err) {
			return db.Article{}, ErrArticleAlreadyExists
		}
		return db.Article{}, fmt.Errorf("failed to create article: %w", err)
	}

	return article, nil
}

func (c *ArticleController) FindByID(ctx context.Context, q db.Querier, id string) (db.Article, error) {
	article, err := q.FindArticleByID(ctx, id)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return db.Article{}, ErrArticleNotFound
		}
		return db.Article{}, fmt.Errorf("failed to find article: %w", err)
	}
	return article, nil
}

// FindByURLOriginal looks up an active article by its original URL. Used by discovery to skip
// articles that already exist (deduplication by url_original).
func (c *ArticleController) FindByURLOriginal(ctx context.Context, q db.Querier, urlOriginal string) (db.Article, error) {
	article, err := q.FindArticleByURLOriginal(ctx, urlOriginal)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return db.Article{}, ErrArticleNotFound
		}
		return db.Article{}, fmt.Errorf("failed to find article by url: %w", err)
	}
	return article, nil
}

// List returns one page of active articles matching the filter, plus the total number of matching
// rows (which is what pagination.total_count reports, so it counts every match, not just this
// page). Filtering and pagination happen in SQL; the two queries share the filter so the page and
// the total can never disagree about what "matching" means.
func (c *ArticleController) List(ctx context.Context, q db.Querier, filter ListArticlesFilter) ([]db.Article, int64, error) {
	articles, err := q.ListArticles(ctx, db.ListArticlesParams{
		Url:        nullableString(filter.URL),
		PageLimit:  filter.Page.Limit(),
		PageOffset: filter.Page.Offset(),
	})
	if err != nil {
		return nil, 0, fmt.Errorf("failed to list articles: %w", err)
	}

	total, err := q.CountArticles(ctx, nullableString(filter.URL))
	if err != nil {
		return nil, 0, fmt.Errorf("failed to count articles: %w", err)
	}

	if articles == nil {
		articles = []db.Article{}
	}
	return articles, total, nil
}

// ListAll returns every active article, for the dev seed scripts. HTTP handlers must use List
// instead: a client always gets a page.
func (c *ArticleController) ListAll(ctx context.Context, q db.Querier) ([]db.Article, error) {
	articles, err := q.ListAllArticles(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to list all articles: %w", err)
	}
	if articles == nil {
		return []db.Article{}, nil
	}
	return articles, nil
}

func (c *ArticleController) Update(ctx context.Context, q db.Querier, id, title, content, urlOriginal string, keywords []string, languageOriginal *string) (db.Article, error) {
	encodedKeywords, err := encodeKeywords(keywords)
	if err != nil {
		return db.Article{}, err
	}

	article, err := q.UpdateArticle(ctx, db.UpdateArticleParams{
		ID:               id,
		Title:            title,
		Content:          content,
		UrlOriginal:      urlOriginal,
		Keywords:         encodedKeywords,
		LanguageOriginal: nullString(languageOriginal),
	})
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return db.Article{}, ErrArticleNotFound
		}
		if isUniqueViolation(err) {
			return db.Article{}, ErrArticleAlreadyExists
		}
		return db.Article{}, fmt.Errorf("failed to update article: %w", err)
	}
	return article, nil
}

// UpdateContent overwrites only the article body, for the 0.45 treatment DB step: right after the
// article is stored, its anchors are rewritten to outbound-link ids and the body is written back. It
// runs inside the same transaction as the insert. Unlike Update it touches nothing else.
func (c *ArticleController) UpdateContent(ctx context.Context, q db.Querier, id, content string) error {
	if err := q.UpdateArticleContent(ctx, db.UpdateArticleContentParams{ID: id, Content: content}); err != nil {
		return fmt.Errorf("failed to update article content: %w", err)
	}
	return nil
}

func (c *ArticleController) SoftDelete(ctx context.Context, q db.Querier, id string) error {
	if err := q.SoftDeleteArticle(ctx, id); err != nil {
		return fmt.Errorf("failed to delete article: %w", err)
	}
	return nil
}

// nullString maps an optional string to sql.NullString: nil (or empty) becomes NULL. Used for
// language_original, which is null when detection fails.
func nullString(s *string) sql.NullString {
	if s == nil || *s == "" {
		return sql.NullString{}
	}
	return sql.NullString{String: *s, Valid: true}
}

// SuggestKeywords produces keyword suggestions for building a feed, drawn from the global article
// pool (articles are public, so no user scoping). It implements the two-strategy contract from the
// roadmap: when the user has already picked keywords, it first tries "related" (keywords that
// co-occur with the picks); if that yields nothing — or if nothing was picked yet — it falls back to
// "popular" (the most common keywords within the recency window). The returned strategy string tells
// the caller which path produced the list. An empty result is legitimate (a brand-new, empty
// database has no keywords to suggest) and is not an error.
//
// since bounds the "popular" query to a recency window; the caller passes the epoch to disable it.
// It does not touch "related", which is topical rather than temporal by design (see the query).
func (c *ArticleController) SuggestKeywords(ctx context.Context, q db.Querier, selected []string, since time.Time, limit int32) ([]schemas.KeywordSuggestion, string, error) {
	// Normalize the picks once into the lowercase JSON array the queries expect. This same encoding
	// drives both "which articles are relevant" and "which keywords to exclude from the output".
	encoded, err := encodeKeywords(selected)
	if err != nil {
		return nil, "", err
	}

	if len(selected) > 0 {
		related, err := q.SuggestRelatedKeywords(ctx, db.SuggestRelatedKeywordsParams{
			Selected:    encoded,
			ResultLimit: limit,
		})
		if err != nil {
			return nil, "", fmt.Errorf("failed to suggest related keywords: %w", err)
		}
		if len(related) > 0 {
			out := make([]schemas.KeywordSuggestion, len(related))
			for i, r := range related {
				out[i] = schemas.KeywordSuggestion{Keyword: r.Keyword, Count: r.Occurrences}
			}
			return out, schemas.KeywordStrategyRelated, nil
		}
	}

	popular, err := q.SuggestPopularKeywords(ctx, db.SuggestPopularKeywordsParams{
		Since:       utctime.New(since),
		Exclude:     encoded,
		ResultLimit: limit,
	})
	if err != nil {
		return nil, "", fmt.Errorf("failed to suggest popular keywords: %w", err)
	}
	out := make([]schemas.KeywordSuggestion, len(popular))
	for i, r := range popular {
		out[i] = schemas.KeywordSuggestion{Keyword: r.Keyword, Count: r.Occurrences}
	}
	return out, schemas.KeywordStrategyPopular, nil
}

// encodeKeywords serializes the keyword slice into the JSONB array stored in the DB. It is the
// single storage choke point for both articles and feeds, so it normalizes every keyword to
// trimmed lowercase (PROJECT.md: "palavras-chave devem ser armazenadas em letras minúsculas").
func encodeKeywords(keywords []string) (json.RawMessage, error) {
	normalized := make([]string, 0, len(keywords))
	for _, k := range keywords {
		normalized = append(normalized, strings.ToLower(strings.TrimSpace(k)))
	}
	data, err := json.Marshal(normalized)
	if err != nil {
		return nil, fmt.Errorf("failed to encode keywords: %w", err)
	}
	return data, nil
}

// DecodeKeywords parses the JSONB array stored in the DB back into a keyword slice.
func DecodeKeywords(encoded json.RawMessage) ([]string, error) {
	if len(encoded) == 0 {
		return []string{}, nil
	}
	var keywords []string
	if err := json.Unmarshal(encoded, &keywords); err != nil {
		return nil, fmt.Errorf("failed to decode keywords: %w", err)
	}
	if keywords == nil {
		keywords = []string{}
	}
	return keywords, nil
}
