package controllers

import (
	"context"
	"database/sql"
	"errors"
	"fmt"

	"github.com/google/uuid"
	db "github.com/nathanap/news-feed-backend/sqlc"
)

var (
	ErrSourceNotFound      = errors.New("source not found")
	ErrSourceAlreadyExists = errors.New("source with this URL already exists")
)

type SourceController struct{}

func NewSourceController() *SourceController {
	return &SourceController{}
}

func (c *SourceController) Create(ctx context.Context, q db.Querier, name, url, urlRss string) (db.Source, error) {
	id, err := uuid.NewV7()
	if err != nil {
		return db.Source{}, fmt.Errorf("failed to generate source ID: %w", err)
	}

	source, err := q.CreateSource(ctx, db.CreateSourceParams{
		ID:     id.String(),
		Name:   name,
		Url:    url,
		UrlRss: urlRss,
	})
	if err != nil {
		if isUniqueViolation(err) {
			return db.Source{}, ErrSourceAlreadyExists
		}
		return db.Source{}, fmt.Errorf("failed to create source: %w", err)
	}

	return source, nil
}

func (c *SourceController) FindByID(ctx context.Context, q db.Querier, id string) (db.Source, error) {
	source, err := q.FindSourceByID(ctx, id)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return db.Source{}, ErrSourceNotFound
		}
		return db.Source{}, fmt.Errorf("failed to find source: %w", err)
	}
	return source, nil
}

func (c *SourceController) List(ctx context.Context, q db.Querier) ([]db.Source, error) {
	sources, err := q.ListSources(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to list sources: %w", err)
	}
	if sources == nil {
		return []db.Source{}, nil
	}
	return sources, nil
}

func (c *SourceController) Update(ctx context.Context, q db.Querier, id, name, url, urlRss string) (db.Source, error) {
	source, err := q.UpdateSource(ctx, db.UpdateSourceParams{
		ID:     id,
		Name:   name,
		Url:    url,
		UrlRss: urlRss,
	})
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return db.Source{}, ErrSourceNotFound
		}
		if isUniqueViolation(err) {
			return db.Source{}, ErrSourceAlreadyExists
		}
		return db.Source{}, fmt.Errorf("failed to update source: %w", err)
	}
	return source, nil
}

// SoftDelete soft-deletes the source and cascades the same soft-delete to every article
// that belongs to it. Running both on the caller's transaction guarantees that no active
// article is ever left pointing to an inactive source.
func (c *SourceController) SoftDelete(ctx context.Context, q db.Querier, id string) error {
	if err := q.SoftDeleteSource(ctx, id); err != nil {
		return fmt.Errorf("failed to delete source: %w", err)
	}

	if err := q.SoftDeleteArticlesBySourceID(ctx, id); err != nil {
		return fmt.Errorf("failed to cascade delete source articles: %w", err)
	}

	return nil
}
