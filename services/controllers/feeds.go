package controllers

import (
	"context"
	"database/sql"
	"errors"
	"fmt"

	"github.com/google/uuid"
	"github.com/nathanap/news-feed-backend/schemas"
	db "github.com/nathanap/news-feed-backend/sqlc"
)

var (
	ErrFeedNotFound     = errors.New("feed not found")
	ErrFeedLimitReached = errors.New("feed limit reached")
)

type FeedController struct{}

func NewFeedController() *FeedController {
	return &FeedController{}
}

// Create enforces the per-user active feed limit before inserting. Counting and inserting
// run on the same querier (the caller's transaction), so the limit check cannot race with
// the insert.
func (c *FeedController) Create(ctx context.Context, q db.Querier, userID, name string, keywords []string) (db.Feed, error) {
	count, err := q.CountActiveFeedsByUser(ctx, userID)
	if err != nil {
		return db.Feed{}, fmt.Errorf("failed to count user feeds: %w", err)
	}
	if count >= schemas.FeedMaxPerUser {
		return db.Feed{}, ErrFeedLimitReached
	}

	id, err := uuid.NewV7()
	if err != nil {
		return db.Feed{}, fmt.Errorf("failed to generate feed ID: %w", err)
	}

	encodedKeywords, err := encodeKeywords(keywords)
	if err != nil {
		return db.Feed{}, err
	}

	feed, err := q.CreateFeed(ctx, db.CreateFeedParams{
		ID:       id.String(),
		Name:     name,
		Keywords: encodedKeywords,
		UserID:   userID,
	})
	if err != nil {
		return db.Feed{}, fmt.Errorf("failed to create feed: %w", err)
	}

	return feed, nil
}

func (c *FeedController) FindByID(ctx context.Context, q db.Querier, id, userID string) (db.Feed, error) {
	feed, err := q.FindFeedByIDAndUser(ctx, db.FindFeedByIDAndUserParams{ID: id, UserID: userID})
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return db.Feed{}, ErrFeedNotFound
		}
		return db.Feed{}, fmt.Errorf("failed to find feed: %w", err)
	}
	return feed, nil
}

func (c *FeedController) List(ctx context.Context, q db.Querier, userID string) ([]db.Feed, error) {
	feeds, err := q.ListFeedsByUser(ctx, userID)
	if err != nil {
		return nil, fmt.Errorf("failed to list feeds: %w", err)
	}
	if feeds == nil {
		return []db.Feed{}, nil
	}
	return feeds, nil
}

// FeedCandidate is a layer-1 judgement candidate: an active feed that shares at least one keyword
// with the article, paired with OverlapCount (how many distinct keywords matched). The overlap is
// the signal the layer-2 triage uses to decide auto-associate / discard / send-to-AI.
type FeedCandidate struct {
	Feed         db.Feed
	OverlapCount int
}

// FindCandidatesByKeywords returns the active feeds (across all users) that share at least one
// keyword with the given article keywords, each with its keyword-overlap count. This is the cheap
// first layer of judgement: it narrows the whole feed set down to plausible candidates before the
// (expensive) AI scoring. Keywords are normalized to the same lowercase JSON representation used for
// storage so equality matches.
func (c *FeedController) FindCandidatesByKeywords(ctx context.Context, q db.Querier, keywords []string) ([]FeedCandidate, error) {
	if len(keywords) == 0 {
		return []FeedCandidate{}, nil
	}

	encodedKeywords, err := encodeKeywords(keywords)
	if err != nil {
		return nil, err
	}

	rows, err := q.FindCandidateFeedsByKeywords(ctx, encodedKeywords)
	if err != nil {
		return nil, fmt.Errorf("failed to find candidate feeds: %w", err)
	}

	candidates := make([]FeedCandidate, 0, len(rows))
	for _, r := range rows {
		candidates = append(candidates, FeedCandidate{
			Feed: db.Feed{
				ID: r.ID, Status: r.Status, Name: r.Name, Keywords: r.Keywords,
				UserID: r.UserID, CreatedAt: r.CreatedAt, ModifiedAt: r.ModifiedAt, RemovedAt: r.RemovedAt,
			},
			OverlapCount: int(r.OverlapCount),
		})
	}
	return candidates, nil
}

func (c *FeedController) Update(ctx context.Context, q db.Querier, id, userID, name string, keywords []string) (db.Feed, error) {
	encodedKeywords, err := encodeKeywords(keywords)
	if err != nil {
		return db.Feed{}, err
	}

	feed, err := q.UpdateFeedByIDAndUser(ctx, db.UpdateFeedByIDAndUserParams{
		Name:     name,
		Keywords: encodedKeywords,
		ID:       id,
		UserID:   userID,
	})
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return db.Feed{}, ErrFeedNotFound
		}
		return db.Feed{}, fmt.Errorf("failed to update feed: %w", err)
	}
	return feed, nil
}

func (c *FeedController) SoftDelete(ctx context.Context, q db.Querier, id, userID string) error {
	if err := q.SoftDeleteFeedByIDAndUser(ctx, db.SoftDeleteFeedByIDAndUserParams{ID: id, UserID: userID}); err != nil {
		return fmt.Errorf("failed to delete feed: %w", err)
	}
	return nil
}
