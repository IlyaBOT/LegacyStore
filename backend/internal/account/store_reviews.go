package account

import (
	"context"
	"database/sql"
	"errors"
	"net"
	"strings"
)

func (s *Store) CreateReview(ctx context.Context, user User, appSlug string, rating int, title, body string, ip net.IP, userAgent string) (*Review, error) {
	if rating < 1 || rating > 5 {
		return nil, ErrInvalidCredential
	}
	appID, err := s.appIDBySlug(ctx, appSlug)
	if err != nil {
		return nil, err
	}
	var review Review
	err = s.db.QueryRowContext(ctx, `
		INSERT INTO reviews (app_id, user_id, rating, title, body)
		VALUES ($1, $2, $3, NULLIF($4, ''), NULLIF($5, ''))
		ON CONFLICT (app_id, user_id) WHERE deleted_at IS NULL
		DO UPDATE SET rating = EXCLUDED.rating, title = EXCLUDED.title, body = EXCLUDED.body, updated_at = now()
		RETURNING id, rating, COALESCE(title, ''), COALESCE(body, ''), created_at::text, updated_at::text
	`, appID, user.ID, rating, strings.TrimSpace(title), strings.TrimSpace(body)).Scan(&review.ID, &review.Rating, &review.Title, &review.Body, &review.CreatedAt, &review.UpdatedAt)
	if err != nil {
		return nil, err
	}
	review.AppSlug = appSlug
	review.UserID = user.ID
	review.Author = displayName(user)
	_ = s.audit(ctx, user.ID, "reviews.write", "review", intString(review.ID), ip, userAgent)
	return &review, nil
}

func (s *Store) UpdateReview(ctx context.Context, user User, reviewID int64, rating int, title, body string, ip net.IP, userAgent string) (*Review, error) {
	if rating < 1 || rating > 5 {
		return nil, ErrInvalidCredential
	}
	allowed, err := s.canChangeReview(ctx, user, reviewID)
	if err != nil {
		return nil, err
	}
	if !allowed {
		return nil, ErrForbidden
	}
	var review Review
	err = s.db.QueryRowContext(ctx, `
		UPDATE reviews
		SET rating = $2, title = NULLIF($3, ''), body = NULLIF($4, '')
		WHERE id = $1 AND deleted_at IS NULL
		RETURNING id, rating, COALESCE(title, ''), COALESCE(body, ''), created_at::text, updated_at::text
	`, reviewID, rating, strings.TrimSpace(title), strings.TrimSpace(body)).Scan(&review.ID, &review.Rating, &review.Title, &review.Body, &review.CreatedAt, &review.UpdatedAt)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	review.UserID = user.ID
	review.Author = displayName(user)
	_ = s.audit(ctx, user.ID, "reviews.update", "review", intString(review.ID), ip, userAgent)
	return &review, nil
}

func (s *Store) DeleteReview(ctx context.Context, user User, reviewID int64, ip net.IP, userAgent string) error {
	allowed, err := s.canChangeReview(ctx, user, reviewID)
	if err != nil {
		return err
	}
	if !allowed {
		return ErrForbidden
	}
	result, err := s.db.ExecContext(ctx, `UPDATE reviews SET deleted_at = now() WHERE id = $1 AND deleted_at IS NULL`, reviewID)
	if err != nil {
		return err
	}
	if n, _ := result.RowsAffected(); n == 0 {
		return ErrNotFound
	}
	return s.audit(ctx, user.ID, "reviews.delete", "review", intString(reviewID), ip, userAgent)
}

func (s *Store) LikeReview(ctx context.Context, user User, reviewID int64) error {
	if !s.reviewExists(ctx, reviewID) {
		return ErrNotFound
	}
	_, err := s.db.ExecContext(ctx, `INSERT INTO review_likes (review_id, user_id) VALUES ($1, $2) ON CONFLICT DO NOTHING`, reviewID, user.ID)
	return err
}

func (s *Store) UnlikeReview(ctx context.Context, userID, reviewID int64) error {
	_, err := s.db.ExecContext(ctx, `DELETE FROM review_likes WHERE review_id = $1 AND user_id = $2`, reviewID, userID)
	return err
}

func (s *Store) CreateReviewReply(ctx context.Context, user User, reviewID int64, body string, ip net.IP, userAgent string) (*ReviewReply, error) {
	body = strings.TrimSpace(body)
	if body == "" {
		return nil, ErrInvalidCredential
	}
	if !s.reviewExists(ctx, reviewID) {
		return nil, ErrNotFound
	}
	var reply ReviewReply
	err := s.db.QueryRowContext(ctx, `
		INSERT INTO review_replies (review_id, user_id, body)
		VALUES ($1, $2, $3)
		RETURNING id, review_id, user_id, body, created_at::text
	`, reviewID, user.ID, body).Scan(&reply.ID, &reply.ReviewID, &reply.UserID, &reply.Body, &reply.CreatedAt)
	if err != nil {
		return nil, err
	}
	reply.Author = displayName(user)
	_ = s.audit(ctx, user.ID, "reviews.reply", "review", intString(reviewID), ip, userAgent)
	return &reply, nil
}

func (s *Store) appIDBySlug(ctx context.Context, slug string) (int64, error) {
	var appID int64
	err := s.db.QueryRowContext(ctx, `SELECT id FROM apps WHERE slug = $1 AND moderation_status = 'approved'`, slug).Scan(&appID)
	if errors.Is(err, sql.ErrNoRows) {
		return 0, ErrNotFound
	}
	return appID, err
}

func (s *Store) reviewExists(ctx context.Context, reviewID int64) bool {
	var exists bool
	_ = s.db.QueryRowContext(ctx, `SELECT EXISTS (SELECT 1 FROM reviews WHERE id = $1 AND deleted_at IS NULL)`, reviewID).Scan(&exists)
	return exists
}

func (s *Store) canChangeReview(ctx context.Context, user User, reviewID int64) (bool, error) {
	if HasRole(user, "admin", "moder") {
		return true, nil
	}
	var ownerID int64
	err := s.db.QueryRowContext(ctx, `SELECT user_id FROM reviews WHERE id = $1 AND deleted_at IS NULL`, reviewID).Scan(&ownerID)
	if errors.Is(err, sql.ErrNoRows) {
		return false, ErrNotFound
	}
	if err != nil {
		return false, err
	}
	return ownerID == user.ID, nil
}

func displayName(user User) string {
	if strings.TrimSpace(user.Nickname) != "" {
		return user.Nickname
	}
	return user.Email
}
