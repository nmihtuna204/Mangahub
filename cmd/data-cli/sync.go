package main

import (
	"context"
	"database/sql"
	"flag"
	"fmt"
	"os"
	"os/signal"

	"mangahub/internal/chapters"
	"mangahub/internal/udp"
	"mangahub/pkg/config"
	"mangahub/pkg/external"
)

// runSyncChapters implements `data-cli sync-chapters`: check MangaDex for
// chapters newer than manga.total_chapters, record them, and send a UDP
// chapter_release to each manga's readers (only to them).
func runSyncChapters(cfg *config.Config, db *sql.DB, args []string) {
	fs := flag.NewFlagSet("sync-chapters", flag.ExitOnError)
	link := fs.Bool("link", false, "look up MangaDex IDs by title for manga that don't have one yet")
	dryRun := fs.Bool("dry-run", false, "show new chapters without recording them or notifying anyone")
	mangaID := fs.String("manga", "", "only check this manga ID")
	limit := fs.Int("limit", 0, "check at most this many manga (0 = all)")
	fs.Parse(args)

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt)
	defer stop()

	udpAddr := fmt.Sprintf("%s:%d", cfg.UDP.Host, cfg.UDP.Port)
	service := chapters.NewService(db, udp.NewBroadcaster(udpAddr))
	source := chapters.NewMangaDexSource(external.NewMangaDexClient(&cfg.MangaDex), cfg.Chapters.Language)
	syncer := chapters.NewSyncer(db, service, source)

	mode := ""
	if *dryRun {
		mode = " (dry run)"
	}
	fmt.Printf("🔄 Checking MangaDex for new chapters%s...\n\n", mode)

	results, err := syncer.Sync(ctx, chapters.SyncOptions{Link: *link, DryRun: *dryRun, MangaID: *mangaID, Limit: *limit})
	released, linked, failed := 0, 0, 0
	for _, r := range results {
		if r.Linked {
			linked++
		}
		switch {
		case r.Released != nil && *dryRun:
			released++
			fmt.Printf("🆕 %-40s %d → %d (would notify readers)\n", r.Title, r.Known, r.Latest)
		case r.Released != nil:
			released++
			fmt.Printf("🆕 %-40s %d → %d, notified %d readers\n", r.Title, r.Known, r.Latest, r.Released.NotifiedReaders)
		case r.Error != "":
			failed++
			fmt.Printf("⚠️  %-40s %s\n", r.Title, r.Error)
		default:
			fmt.Printf("✓  %-40s up to date (MangaDex has %d, we know %d)\n", r.Title, r.Latest, r.Known)
		}
	}
	if err != nil {
		fmt.Printf("\n❌ Sync stopped: %v\n", err)
	}

	fmt.Printf("\nChecked %d manga: %d new chapters, %d newly linked to MangaDex, %d skipped\n",
		len(results), released, linked, failed)
	if len(results) == 0 && !*link {
		fmt.Println("No manga is linked to MangaDex yet. Run with --link to look them up by title.")
	}
	if released > 0 && !*dryRun {
		fmt.Printf("Notifications sent through the UDP server at %s\n", udpAddr)
	}
}
