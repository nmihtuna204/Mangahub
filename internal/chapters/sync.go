package chapters

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"

	"mangahub/pkg/logger"
	"mangahub/pkg/models"
)

// Source reports the newest chapter of a manga on an external site.
type Source interface {
	// Name identifies the source in logs and results ("mangadex")
	Name() string
	// FindID looks up the site's ID for a manga by title; "" if there is no
	// confident match
	FindID(ctx context.Context, title string) (string, error)
	// LatestChapter returns the highest chapter number available (0 if none)
	LatestChapter(ctx context.Context, externalID string) (int, error)
}

// SyncOptions controls a sync run.
type SyncOptions struct {
	Link    bool   // look up and save external IDs for manga that have none
	DryRun  bool   // report new chapters without recording or notifying
	MangaID string // only check this manga
	Limit   int    // check at most this many manga (0 = all)
}

// SyncResult is the outcome for one manga.
type SyncResult struct {
	MangaID    string   `json:"manga_id"`
	Title      string   `json:"title"`
	ExternalID string   `json:"external_id,omitempty"`
	Linked     bool     `json:"linked,omitempty"` // external ID found during this run
	Known      int      `json:"known_chapters"`   // total_chapters before the run
	Latest     int      `json:"latest_chapter"`   // reported by the source
	Released   *Release `json:"released,omitempty"`
	Error      string   `json:"error,omitempty"`
}

// Syncer checks a Source for chapters newer than what the database knows and
// releases them.
type Syncer struct {
	db      *sql.DB
	service *Service
	source  Source
}

// NewSyncer creates a syncer that releases new chapters through service.
func NewSyncer(db *sql.DB, service *Service, source Source) *Syncer {
	return &Syncer{db: db, service: service, source: source}
}

type candidate struct {
	id, title, externalID string
	known                 int
}

// Sync checks each manga (that has, or with Link gets, an external ID) and
// releases any newer chapter. One manga failing doesn't stop the run.
func (sy *Syncer) Sync(ctx context.Context, opts SyncOptions) ([]SyncResult, error) {
	candidates, err := sy.candidates(ctx, opts)
	if err != nil {
		return nil, err
	}

	results := make([]SyncResult, 0, len(candidates))
	for _, c := range candidates {
		if ctx.Err() != nil {
			return results, ctx.Err()
		}
		results = append(results, sy.syncOne(ctx, c, opts))
	}
	return results, nil
}

func (sy *Syncer) syncOne(ctx context.Context, c candidate, opts SyncOptions) SyncResult {
	res := SyncResult{MangaID: c.id, Title: c.title, ExternalID: c.externalID, Known: c.known}
	fail := func(err error) SyncResult {
		res.Error = err.Error()
		logger.Warnf("chapter sync: %s: %v", c.title, err)
		return res
	}

	if res.ExternalID == "" {
		id, err := sy.source.FindID(ctx, c.title)
		if err != nil {
			return fail(fmt.Errorf("look up on %s: %w", sy.source.Name(), err))
		}
		if id == "" {
			res.Error = "no confident match on " + sy.source.Name()
			return res
		}
		res.ExternalID, res.Linked = id, true
		if !opts.DryRun {
			if err := sy.saveLink(ctx, c.id, id); err != nil {
				return fail(err)
			}
		}
	}

	latest, err := sy.source.LatestChapter(ctx, res.ExternalID)
	if err != nil {
		return fail(fmt.Errorf("latest chapter: %w", err))
	}
	res.Latest = latest
	if !opts.DryRun {
		sy.markSynced(ctx, c.id)
	}

	if latest <= c.known {
		return res
	}
	if opts.DryRun {
		res.Released = &Release{MangaID: c.id, Title: c.title, PreviousLatest: c.known, Chapter: latest}
		return res
	}

	rel, err := sy.service.Release(ctx, c.id, latest)
	var appErr *models.AppError
	if errors.As(err, &appErr) && appErr.StatusCode == 409 {
		return res // someone released it in the meantime
	}
	if rel != nil {
		res.Released = rel
	}
	if err != nil {
		return fail(err)
	}
	return res
}

// candidates loads the manga to check, fully, before any network call, so no
// database connection is held while waiting on the external API.
func (sy *Syncer) candidates(ctx context.Context, opts SyncOptions) ([]candidate, error) {
	query := `
		SELECT m.id, m.title, m.total_chapters, COALESCE(e.mangadex_id, '')
		FROM manga m
		LEFT JOIN manga_external_ids e ON e.manga_id = m.id
		WHERE (? = '' OR m.id = ?)`
	if !opts.Link {
		query += ` AND COALESCE(e.mangadex_id, '') != ''`
	}
	query += ` ORDER BY m.title`
	args := []interface{}{opts.MangaID, opts.MangaID}
	if opts.Limit > 0 {
		query += ` LIMIT ?`
		args = append(args, opts.Limit)
	}

	rows, err := sy.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("load manga to sync: %w", err)
	}
	defer rows.Close()

	var out []candidate
	for rows.Next() {
		var c candidate
		if err := rows.Scan(&c.id, &c.title, &c.known, &c.externalID); err != nil {
			return nil, fmt.Errorf("scan manga to sync: %w", err)
		}
		out = append(out, c)
	}
	return out, rows.Err()
}

func (sy *Syncer) saveLink(ctx context.Context, mangaID, externalID string) error {
	now := time.Now()
	_, err := sy.db.ExecContext(ctx, `
		INSERT INTO manga_external_ids (manga_id, mangadex_id, primary_source, created_at, updated_at)
		VALUES (?, ?, 'mangadex', ?, ?)
		ON CONFLICT(manga_id) DO UPDATE SET mangadex_id = excluded.mangadex_id, updated_at = excluded.updated_at`,
		mangaID, externalID, now, now)
	if err != nil {
		return fmt.Errorf("save %s ID: %w", sy.source.Name(), err)
	}
	return nil
}

func (sy *Syncer) markSynced(ctx context.Context, mangaID string) {
	_, _ = sy.db.ExecContext(ctx, `UPDATE manga_external_ids SET last_synced_at = ? WHERE manga_id = ?`, time.Now(), mangaID)
}

// RunEvery syncs every interval until ctx is canceled (linking unknown manga
// as it goes). Used by the API server when chapters.sync_interval is set.
func (sy *Syncer) RunEvery(ctx context.Context, interval time.Duration) {
	ticker := time.NewTicker(interval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			results, err := sy.Sync(ctx, SyncOptions{Link: true})
			if err != nil && ctx.Err() == nil {
				logger.Errorf("chapter sync: %v", err)
			}
			released := 0
			for _, r := range results {
				if r.Released != nil {
					released++
				}
			}
			logger.Infof("chapter sync: checked %d manga, %d new chapters released", len(results), released)
		}
	}
}
