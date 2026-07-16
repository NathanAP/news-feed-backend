package controllers

import (
	"context"
	"fmt"

	"github.com/google/uuid"
	db "github.com/nathanap/news-feed-backend/sqlc"
)

type ArticleFeedController struct{}

func NewArticleFeedController() *ArticleFeedController {
	return &ArticleFeedController{}
}

// Create inserts a new articles_feeds record. This is used by the judgement process (0.17)
// and by test fixtures. The caller is responsible for the transaction boundary.
func (c *ArticleFeedController) Create(ctx context.Context, q db.Querier, articleID, feedID string) (db.ArticlesFeed, error) {
	id, err := uuid.NewV7()
	if err != nil {
		return db.ArticlesFeed{}, fmt.Errorf("failed to generate articles_feeds ID: %w", err)
	}

	af, err := q.CreateArticleFeed(ctx, db.CreateArticleFeedParams{
		ID:        id.String(),
		ArticleID: articleID,
		FeedID:    feedID,
	})
	if err != nil {
		return db.ArticlesFeed{}, fmt.Errorf("failed to create articles_feeds record: %w", err)
	}

	return af, nil
}

// FindByArticleAndUser returns all articles_feeds records for the given article that belong
// to the requesting user (via JOIN with feeds). Records whose feed is soft-deleted are
// excluded by the join condition, becoming naturally invisible.
func (c *ArticleFeedController) FindByArticleAndUser(ctx context.Context, q db.Querier, articleID, userID string) ([]db.ArticlesFeed, error) {
	records, err := q.FindArticleFeedsByArticleAndUser(ctx, db.FindArticleFeedsByArticleAndUserParams{
		UserID:    userID,
		ArticleID: articleID,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to find articles_feeds records: %w", err)
	}
	if records == nil {
		return []db.ArticlesFeed{}, nil
	}
	return records, nil
}

// ListArticlesByFeedForUser returns the active articles of a feed (each with its is_read state for
// that feed), scoped to the requesting user. It does not distinguish a feed that is empty from one
// that does not belong to the user — both yield an empty slice — so the caller must check feed
// ownership first (via FeedController.FindByID) to answer 404 vs 200. is_read / date filtering and
// pagination are applied by the caller over this base set (same in-memory pattern as the other
// list endpoints).
func (c *ArticleFeedController) ListArticlesByFeedForUser(ctx context.Context, q db.Querier, feedID, userID string) ([]db.ListArticlesByFeedForUserRow, error) {
	rows, err := q.ListArticlesByFeedForUser(ctx, db.ListArticlesByFeedForUserParams{
		UserID: userID,
		FeedID: feedID,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to list articles by feed: %w", err)
	}
	if rows == nil {
		return []db.ListArticlesByFeedForUserRow{}, nil
	}
	return rows, nil
}

// CountUnreadByFeedForUser returns, per active feed owned by the user, how many unread articles it
// has. Feeds with zero unread articles are omitted (the query only groups feeds that have at least
// one), so the result is exactly the set of feeds that currently have new articles. The count is
// computed entirely in SQL.
func (c *ArticleFeedController) CountUnreadByFeedForUser(ctx context.Context, q db.Querier, userID string) ([]db.CountUnreadArticlesByFeedForUserRow, error) {
	rows, err := q.CountUnreadArticlesByFeedForUser(ctx, userID)
	if err != nil {
		return nil, fmt.Errorf("failed to count unread articles by feed: %w", err)
	}
	if rows == nil {
		return []db.CountUnreadArticlesByFeedForUserRow{}, nil
	}
	return rows, nil
}

// MarkAsRead marks is_read = 1 on all unread articles_feeds records for the given article
// and user. Idempotent: already-read records are not touched. Returns whether any records
// were found (true = user has at least one feed containing the article).
func (c *ArticleFeedController) MarkAsRead(ctx context.Context, q db.Querier, articleID, userID string) (bool, error) {
	// Check first so we can tell the endpoint whether the user has this article in any feed.
	records, err := c.FindByArticleAndUser(ctx, q, articleID, userID)
	if err != nil {
		return false, err
	}
	if len(records) == 0 {
		return false, nil
	}

	if err := q.MarkArticleAsReadForUser(ctx, db.MarkArticleAsReadForUserParams{
		ArticleID: articleID,
		UserID:    userID,
	}); err != nil {
		return true, fmt.Errorf("failed to mark article as read: %w", err)
	}

	return true, nil
}

// IsReadState derives the is_read state for an article from the perspective of a user.
// Returns nil when the article is not in any of the user's feeds.
// Returns false when in at least one feed and at least one is unread.
// Returns true when all associated feeds have is_read = true.
func IsReadState(records []db.ArticlesFeed) *bool {
	if len(records) == 0 {
		return nil
	}
	for _, r := range records {
		if !r.IsRead {
			f := false
			return &f
		}
	}
	t := true
	return &t
}
