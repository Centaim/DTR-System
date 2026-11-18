// Package retention deletes clock-in photos once the agency's retention period ends.
// The attendance log itself is never deleted, and employees on legal hold are skipped.
package retention

import (
	"context"
	"log"
	"time"

	"dtr/server/internal/settings"
	"dtr/server/internal/storage"

	"github.com/jackc/pgx/v5/pgxpool"
)

type Service struct {
	pool     *pgxpool.Pool
	store    storage.Store
	settings *settings.Store
}

func New(pool *pgxpool.Pool, store storage.Store, s *settings.Store) *Service {
	return &Service{pool: pool, store: store, settings: s}
}

// RunOnce purges up to one batch and returns how many photos were removed.
// A retention period of 0 means photos are kept.
func (s *Service) RunOnce(ctx context.Context) (int, error) {
	days := s.settings.Int(settings.PhotoRetentionDays)
	if days <= 0 {
		return 0, nil
	}
	rows, err := s.pool.Query(ctx, `
		SELECT l.id, l.photo_key FROM attendance_logs l
		 WHERE l.recorded_at < now() - ($1 * interval '1 day')
		   AND NOT EXISTS (SELECT 1 FROM photo_purges p WHERE p.log_id = l.id)
		   AND NOT EXISTS (SELECT 1 FROM legal_holds h WHERE h.employee_id = l.employee_id AND h.released_at IS NULL)
		 ORDER BY l.id LIMIT 500`, days)
	if err != nil {
		return 0, err
	}
	type item struct {
		id  int64
		key string
	}
	var items []item
	for rows.Next() {
		var it item
		if err := rows.Scan(&it.id, &it.key); err != nil {
			rows.Close()
			return 0, err
		}
		items = append(items, it)
	}
	rows.Close()
	if err := rows.Err(); err != nil {
		return 0, err
	}

	n := 0
	for _, it := range items {
		if err := s.store.Delete(ctx, it.key); err != nil {
			log.Printf("retention: log %d: %v", it.id, err)
			continue
		}
		if _, err := s.pool.Exec(ctx, `INSERT INTO photo_purges (log_id) VALUES ($1) ON CONFLICT DO NOTHING`, it.id); err != nil {
			return n, err
		}
		n++
	}
	if n > 0 {
		_, err = s.pool.Exec(ctx, `INSERT INTO audit_logs (action, target) VALUES ('photo.purge', 'count:' || $1::text)`, n)
	}
	return n, err
}

// Loop runs the purge every hour until ctx ends.
func (s *Service) Loop(ctx context.Context) {
	t := time.NewTicker(time.Hour)
	defer t.Stop()
	for {
		if _, err := s.RunOnce(ctx); err != nil && ctx.Err() == nil {
			log.Printf("retention: %v", err)
		}
		select {
		case <-ctx.Done():
			return
		case <-t.C:
		}
	}
}
