-- Fix genres migration - handle JSON arrays properly

-- Clear existing incorrect data
DELETE FROM manga_genres;
DELETE FROM genres WHERE name NOT IN (
    SELECT DISTINCT TRIM(REPLACE(REPLACE(REPLACE(REPLACE(REPLACE(
        value, '[', ''), ']', ''), '"', ''), ',', ''), ' ', ' '))
    FROM (
        WITH RECURSIVE chars(c, pos) AS (
            SELECT '', 0
            UNION ALL
            SELECT SUBSTR(genres, pos+1, 1), pos+1 
            FROM manga, chars 
            WHERE pos < LENGTH(manga.genres) AND manga.genres IS NOT NULL
        )
        SELECT SUBSTR(genres, pos, 1) as value
        FROM manga, chars
        WHERE manga.genres IS NOT NULL
    )
);

-- Use JSON functions to extract genres properly
-- First, delete old incorrect data
DELETE FROM genres;
DELETE FROM manga_genres;

-- Insert genres from JSON arrays using json_each
INSERT OR IGNORE INTO genres (id, name, slug, created_at)
SELECT 
    LOWER(REPLACE(REPLACE(value, ' ', '-'), '"', '')) as id,
    TRIM(REPLACE(value, '"', '')) as name,
    LOWER(REPLACE(REPLACE(value, ' ', '-'), '"', '')) as slug,
    CURRENT_TIMESTAMP
FROM (
    SELECT DISTINCT TRIM(json_extract(manga.genres, '$[' || idx.value || ']')) as value
    FROM manga, json_array_length(manga.genres) AS jsonlen,
         (SELECT 0 AS value UNION SELECT 1 UNION SELECT 2 UNION SELECT 3 UNION SELECT 4 
          UNION SELECT 5 UNION SELECT 6 UNION SELECT 7 UNION SELECT 8 UNION SELECT 9 UNION SELECT 10) AS idx
    WHERE manga.genres IS NOT NULL 
      AND idx.value < jsonlen.value
      AND json_extract(manga.genres, '$[' || idx.value || ']') IS NOT NULL
)
WHERE value IS NOT NULL AND TRIM(value) != '';

-- Populate manga_genres from JSON
INSERT OR IGNORE INTO manga_genres (id, manga_id, genre_id, created_at)
SELECT DISTINCT
    m.id || '-' || LOWER(REPLACE(REPLACE(g_val, ' ', '-'), '"', '')) as id,
    m.id,
    LOWER(REPLACE(REPLACE(g_val, ' ', '-'), '"', '')) as genre_id,
    CURRENT_TIMESTAMP
FROM manga m
CROSS JOIN (
    SELECT 0 AS idx UNION SELECT 1 UNION SELECT 2 UNION SELECT 3 UNION SELECT 4 
    UNION SELECT 5 UNION SELECT 6 UNION SELECT 7 UNION SELECT 8 UNION SELECT 9 UNION SELECT 10
) AS indices
CROSS JOIN (
    SELECT TRIM(json_extract(m.genres, '$[' || indices.idx || ']')) as g_val
    FROM json_array_length(m.genres) AS jlen
    WHERE m.genres IS NOT NULL 
      AND indices.idx < jlen.value
      AND json_extract(m.genres, '$[' || indices.idx || ']') IS NOT NULL
) g
WHERE m.genres IS NOT NULL 
  AND g.g_val IS NOT NULL 
  AND TRIM(g.g_val) != '';

-- Verify the migration
SELECT COUNT(*) as total_genres FROM genres;
SELECT COUNT(*) as total_manga_genres FROM manga_genres;
SELECT m.title, g.name FROM manga m 
JOIN manga_genres mg ON m.id = mg.manga_id 
JOIN genres g ON mg.genre_id = g.id 
LIMIT 10;
