package controllers

import (
	"context"
	"fmt"

	db "github.com/nathanap/news-feed-backend/sqlc"
)

// ArticleOutboundLinkController is the intermediary for article_outbound_links: it composes into the
// treatment persistence transaction and the read-time swap. Like every controller it takes a
// db.Querier and never commits — the caller owns the transaction.
type ArticleOutboundLinkController struct{}

func NewArticleOutboundLinkController() *ArticleOutboundLinkController {
	return &ArticleOutboundLinkController{}
}

// Create persists one outbound link. The id is passed in, not generated here: it is the same id the
// outboundlinks service already wrote into the article body's anchor, so body and row must agree. href
// is stored as-is (already token-normalized for internal links by the service).
func (c *ArticleOutboundLinkController) Create(ctx context.Context, q db.Querier, id, articleID, href string) (db.ArticleOutboundLink, error) {
	link, err := q.CreateArticleOutboundLink(ctx, db.CreateArticleOutboundLinkParams{
		ID:        id,
		ArticleID: articleID,
		Href:      href,
	})
	if err != nil {
		return db.ArticleOutboundLink{}, fmt.Errorf("failed to create outbound link: %w", err)
	}
	return link, nil
}

// ListByArticleIDs returns every outbound link of the given articles in one batch (for the read-time
// swap of a whole page — no N+1). An empty input short-circuits to an empty slice.
func (c *ArticleOutboundLinkController) ListByArticleIDs(ctx context.Context, q db.Querier, articleIDs []string) ([]db.ArticleOutboundLink, error) {
	if len(articleIDs) == 0 {
		return []db.ArticleOutboundLink{}, nil
	}
	links, err := q.ListArticleOutboundLinksByArticleIDs(ctx, articleIDs)
	if err != nil {
		return nil, fmt.Errorf("failed to list outbound links: %w", err)
	}
	if links == nil {
		links = []db.ArticleOutboundLink{}
	}
	return links, nil
}

// Retarget repoints every row currently pointing at oldHref to newHref. Used for retroactive linking
// (a new article claims its external URL) and the pre-remove cleanup (fall an internal link back to
// the source before the article vanishes).
func (c *ArticleOutboundLinkController) Retarget(ctx context.Context, q db.Querier, oldHref, newHref string) error {
	if err := q.RetargetArticleOutboundLinksByHref(ctx, db.RetargetArticleOutboundLinksByHrefParams{
		OldHref: oldHref,
		NewHref: newHref,
	}); err != nil {
		return fmt.Errorf("failed to retarget outbound links: %w", err)
	}
	return nil
}

// DeleteByArticleID hard-deletes every outbound link of an article (hard-remove cascade). Dormant
// today — articles are only soft-removed — but kept ready in code.
func (c *ArticleOutboundLinkController) DeleteByArticleID(ctx context.Context, q db.Querier, articleID string) error {
	if err := q.DeleteArticleOutboundLinksByArticleID(ctx, articleID); err != nil {
		return fmt.Errorf("failed to delete outbound links: %w", err)
	}
	return nil
}

// OutboundLinksByID indexes outbound links by their id, producing the map the read-time swap
// (outboundlinks.Resolve) consumes. Ids are globally unique, so links from several articles can share
// one map — a page of articles is resolved against a single batch. Kept here so the outboundlinks
// package stays free of the sqlc types.
func OutboundLinksByID(links []db.ArticleOutboundLink) map[string]string {
	byID := make(map[string]string, len(links))
	for _, l := range links {
		byID[l.ID] = l.Href
	}
	return byID
}
