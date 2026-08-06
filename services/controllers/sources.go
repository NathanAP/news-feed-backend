package controllers

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"github.com/nathanap/news-feed-backend/services/outboundlinks"

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

// List returns one page of active sources matching the filter, plus the total number of matching
// rows (which is what pagination.total_count reports, so it counts every match, not just this
// page). Filtering and pagination happen in SQL; the two queries share the filter so the page and
// the total can never disagree about what "matching" means.
func (c *SourceController) List(ctx context.Context, q db.Querier, filter ListSourcesFilter) ([]db.Source, int64, error) {
	sources, err := q.ListSources(ctx, db.ListSourcesParams{
		Url:        nullableString(filter.URL),
		Name:       nullableString(filter.Name),
		PageLimit:  filter.Page.Limit(),
		PageOffset: filter.Page.Offset(),
	})
	if err != nil {
		return nil, 0, fmt.Errorf("failed to list sources: %w", err)
	}

	total, err := q.CountSources(ctx, db.CountSourcesParams{
		Url:  nullableString(filter.URL),
		Name: nullableString(filter.Name),
	})
	if err != nil {
		return nil, 0, fmt.Errorf("failed to count sources: %w", err)
	}

	if sources == nil {
		sources = []db.Source{}
	}
	return sources, total, nil
}

// ListAll returns every active source, for the internal batch consumers (the CRON discovery sweep
// and the dev seed scripts). HTTP handlers must use List instead: a client always gets a page.
func (c *SourceController) ListAll(ctx context.Context, q db.Querier) ([]db.Source, error) {
	sources, err := q.ListAllSources(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to list all sources: %w", err)
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

	// Pre-remove cleanup for the cascade, BEFORE the articles go away (0.48.3). A cascaded article is
	// still a removed article, so every outbound link pointing at its internal page must fall back to
	// the source URL — otherwise older articles keep anchors to a page that answers 404, since reads
	// exclude removed articles. `DELETE /v1/articles/{id}` got this in 0.46.1; the cascade did not,
	// and fixing only the symmetric path is what left the gap.
	//
	// Lives here rather than in the endpoint so both ways of removing an article go through the same
	// obligation, per the convention that mandatory dependency flows belong in the controller.
	if err := q.RetargetArticleOutboundLinksBySourceID(ctx, db.RetargetArticleOutboundLinksBySourceIDParams{
		SourceID:           id,
		InternalHrefPrefix: outboundlinks.InternalHrefPrefix(),
	}); err != nil {
		return fmt.Errorf("failed to retarget outbound links of source articles: %w", err)
	}

	if err := q.SoftDeleteArticlesBySourceID(ctx, id); err != nil {
		return fmt.Errorf("failed to cascade delete source articles: %w", err)
	}

	return nil
}
