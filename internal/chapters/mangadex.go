package chapters

import (
	"context"
	"math"
	"strconv"
	"strings"
	"unicode"

	"mangahub/pkg/external"
)

// MangaDexSource reads chapter information from the MangaDex API. Jikan/MAL
// isn't used here: it only reports chapter counts for finished series.
type MangaDexSource struct {
	client   *external.MangaDexClient
	language string
}

// NewMangaDexSource counts chapters translated into language (e.g. "en").
func NewMangaDexSource(client *external.MangaDexClient, language string) *MangaDexSource {
	if language == "" {
		language = "en"
	}
	return &MangaDexSource{client: client, language: language}
}

// Name implements Source.
func (s *MangaDexSource) Name() string { return "mangadex" }

// FindID implements Source: the first search result whose title or an
// alternative title equals ours (ignoring case and punctuation).
func (s *MangaDexSource) FindID(ctx context.Context, title string) (string, error) {
	res, err := s.client.SearchManga(ctx, title, 5, 0)
	if err != nil {
		return "", err
	}
	want := normalizeTitle(title)
	for _, m := range res.Data {
		for _, t := range m.Attributes.Title {
			if normalizeTitle(t) == want {
				return m.ID, nil
			}
		}
		for _, alt := range m.Attributes.AltTitles {
			for _, t := range alt {
				if normalizeTitle(t) == want {
					return m.ID, nil
				}
			}
		}
	}
	return "", nil
}

// LatestChapter implements Source.
func (s *MangaDexSource) LatestChapter(ctx context.Context, mangaDexID string) (int, error) {
	number, err := s.client.LatestChapterNumber(ctx, mangaDexID, s.language)
	if err != nil {
		return 0, err
	}
	return parseChapter(number), nil
}

// parseChapter turns MangaDex chapter numbers ("1194", "1095.5", "" for
// oneshots) into a whole chapter count.
func parseChapter(s string) int {
	f, err := strconv.ParseFloat(strings.TrimSpace(s), 64)
	if err != nil || f < 0 {
		return 0
	}
	return int(math.Floor(f))
}

// normalizeTitle lowercases a title and drops everything but letters and digits.
func normalizeTitle(s string) string {
	var b strings.Builder
	for _, r := range strings.ToLower(s) {
		if unicode.IsLetter(r) || unicode.IsDigit(r) {
			b.WriteRune(r)
		}
	}
	return b.String()
}
