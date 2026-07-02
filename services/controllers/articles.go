package controllers

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	"github.com/google/uuid"
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
	// status = 1 AND removed_at IS NULL, so a soft-deleted source resolves to not-found.
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
		if strings.Contains(err.Error(), "UNIQUE constraint failed") {
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

func (c *ArticleController) List(ctx context.Context, q db.Querier) ([]db.Article, error) {
	articles, err := q.ListArticles(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to list articles: %w", err)
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
		if strings.Contains(err.Error(), "UNIQUE constraint failed") {
			return db.Article{}, ErrArticleAlreadyExists
		}
		return db.Article{}, fmt.Errorf("failed to update article: %w", err)
	}
	return article, nil
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

// encodeKeywords serializes the keyword slice into the JSON array TEXT stored in the DB. It is the
// single storage choke point for both articles and feeds, so it normalizes every keyword to
// trimmed lowercase (PROJECT.md: "palavras-chave devem ser armazenadas em letras minúsculas").
func encodeKeywords(keywords []string) (string, error) {
	normalized := make([]string, 0, len(keywords))
	for _, k := range keywords {
		normalized = append(normalized, strings.ToLower(strings.TrimSpace(k)))
	}
	data, err := json.Marshal(normalized)
	if err != nil {
		return "", fmt.Errorf("failed to encode keywords: %w", err)
	}
	return string(data), nil
}

// DecodeKeywords parses the JSON array TEXT stored in the DB back into a keyword slice.
func DecodeKeywords(encoded string) ([]string, error) {
	if encoded == "" {
		return []string{}, nil
	}
	var keywords []string
	if err := json.Unmarshal([]byte(encoded), &keywords); err != nil {
		return nil, fmt.Errorf("failed to decode keywords: %w", err)
	}
	if keywords == nil {
		keywords = []string{}
	}
	return keywords, nil
}
