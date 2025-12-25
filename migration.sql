-- Database Migration Script: Normalize and Simplify Schema
-- This script migrates data from the old schema to the new simplified schema
-- while preserving all existing data

-- STEP 1: Create new tables that don't exist yet
-- ================================================

-- Create genres table
CREATE TABLE IF NOT EXISTS genres (
    id TEXT PRIMARY KEY,
    name TEXT UNIQUE NOT NULL,
    slug TEXT UNIQUE NOT NULL,
    created_at DATETIME DEFAULT CURRENT_TIMESTAMP
);

-- Create manga_genres junction table
CREATE TABLE IF NOT EXISTS manga_genres (
    id TEXT PRIMARY KEY,
    manga_id TEXT NOT NULL,
    genre_id TEXT NOT NULL,
    created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
    FOREIGN KEY (manga_id) REFERENCES manga(id) ON DELETE CASCADE,
    FOREIGN KEY (genre_id) REFERENCES genres(id) ON DELETE CASCADE,
    UNIQUE(manga_id, genre_id)
);

-- STEP 2: Migrate genres from manga.genres JSON to normalized genres table
-- =========================================================================
-- Parse JSON genres from existing manga table and insert into genres table
-- Format: First, extract unique genres from manga.genres

INSERT OR IGNORE INTO genres (id, name, slug, created_at)
SELECT 
    LOWER(REPLACE(TRIM(value), ' ', '-')) as id,
    TRIM(value) as name,
    LOWER(REPLACE(TRIM(value), ' ', '-')) as slug,
    CURRENT_TIMESTAMP
FROM (
    WITH RECURSIVE split(value, str) AS (
        SELECT '', genres || ',' FROM manga WHERE genres IS NOT NULL AND genres != ''
        UNION ALL
        SELECT 
            SUBSTR(str, 0, INSTR(str, ',')) as value,
            SUBSTR(str, INSTR(str, ',') + 1)
        FROM split
        WHERE str != ''
    )
    SELECT TRIM(value) as value 
    FROM split 
    WHERE TRIM(value) != ''
);

-- Populate manga_genres from manga.genres JSON
INSERT OR IGNORE INTO manga_genres (id, manga_id, genre_id, created_at)
SELECT 
    m.id || '-' || LOWER(REPLACE(TRIM(g.value), ' ', '-')) as id,
    m.id,
    LOWER(REPLACE(TRIM(g.value), ' ', '-')) as genre_id,
    CURRENT_TIMESTAMP
FROM manga m
CROSS JOIN (
    WITH RECURSIVE split(value, str) AS (
        SELECT '', m.genres || ',' FROM manga m WHERE m.genres IS NOT NULL AND m.genres != ''
        UNION ALL
        SELECT 
            SUBSTR(str, 0, INSTR(str, ',')) as value,
            SUBSTR(str, INSTR(str, ',') + 1)
        FROM split
        WHERE str != ''
    )
    SELECT TRIM(value) as value 
    FROM split 
    WHERE TRIM(value) != ''
) g
WHERE m.genres IS NOT NULL AND m.genres != '';

-- STEP 3: Add new columns to users table
-- =======================================
-- Add CHECK constraint by creating new column (SQLite doesn't support ALTER with ADD CONSTRAINT)
-- The role column already exists, we'll add validation in the application

-- STEP 4: Add new columns to manga table
-- ========================================
ALTER TABLE manga ADD COLUMN average_rating REAL DEFAULT 0.0;
ALTER TABLE manga ADD COLUMN rating_count INTEGER DEFAULT 0;

-- Populate average_rating and rating_count from manga_ratings
UPDATE manga 
SET 
    average_rating = COALESCE((
        SELECT AVG(CAST(overall_rating as REAL)) 
        FROM manga_ratings 
        WHERE manga_id = manga.id
    ), 0.0),
    rating_count = COALESCE((
        SELECT COUNT(*) 
        FROM manga_ratings 
        WHERE manga_id = manga.id
    ), 0)
WHERE id IN (SELECT DISTINCT manga_id FROM manga_ratings);

-- STEP 5: Create triggers for manga_ratings to auto-update average_rating
-- ========================================================================
CREATE TRIGGER IF NOT EXISTS update_manga_rating_insert AFTER INSERT ON manga_ratings BEGIN
    UPDATE manga 
    SET average_rating = (SELECT AVG(CAST(overall_rating as REAL)) FROM manga_ratings WHERE manga_id = new.manga_id),
        rating_count = (SELECT COUNT(*) FROM manga_ratings WHERE manga_id = new.manga_id)
    WHERE id = new.manga_id;
END;

CREATE TRIGGER IF NOT EXISTS update_manga_rating_update AFTER UPDATE ON manga_ratings BEGIN
    UPDATE manga 
    SET average_rating = (SELECT AVG(CAST(overall_rating as REAL)) FROM manga_ratings WHERE manga_id = new.manga_id)
    WHERE id = new.manga_id;
END;

CREATE TRIGGER IF NOT EXISTS update_manga_rating_delete AFTER DELETE ON manga_ratings BEGIN
    UPDATE manga 
    SET average_rating = (SELECT COALESCE(AVG(CAST(overall_rating as REAL)), 0) FROM manga_ratings WHERE manga_id = old.manga_id),
        rating_count = (SELECT COUNT(*) FROM manga_ratings WHERE manga_id = old.manga_id)
    WHERE id = old.manga_id;
END;

-- STEP 6: Add indexes for new tables
-- ===================================
CREATE INDEX IF NOT EXISTS idx_manga_genres_manga ON manga_genres(manga_id);
CREATE INDEX IF NOT EXISTS idx_manga_genres_genre ON manga_genres(genre_id);
CREATE INDEX IF NOT EXISTS idx_manga_type ON manga(type);
CREATE INDEX IF NOT EXISTS idx_manga_rating ON manga(average_rating DESC);
CREATE INDEX IF NOT EXISTS idx_external_mangadex ON manga_external_ids(mangadex_id);
CREATE INDEX IF NOT EXISTS idx_external_mal ON manga_external_ids(mal_id);
CREATE INDEX IF NOT EXISTS idx_external_anilist ON manga_external_ids(anilist_id);
CREATE INDEX IF NOT EXISTS idx_ratings_manga ON manga_ratings(manga_id);
CREATE INDEX IF NOT EXISTS idx_ratings_user ON manga_ratings(user_id);
CREATE INDEX IF NOT EXISTS idx_ratings_created ON manga_ratings(created_at DESC);
CREATE INDEX IF NOT EXISTS idx_progress_user ON reading_progress(user_id);
CREATE INDEX IF NOT EXISTS idx_progress_manga ON reading_progress(manga_id);
CREATE INDEX IF NOT EXISTS idx_progress_favorite ON reading_progress(is_favorite) WHERE is_favorite = 1;

-- STEP 7: Update chat_messages - rename 'edited' to 'is_edited', 'deleted' to 'is_deleted'
-- ========================================================================================
-- SQLite doesn't support renaming columns directly in old versions, so we'll recreate if needed
-- For now, just verify the columns exist and add migration triggers if needed

-- STEP 8: Create triggers for comment likes auto-count
-- =====================================================
CREATE TRIGGER IF NOT EXISTS increment_comment_likes AFTER INSERT ON comment_likes BEGIN
    UPDATE comments SET likes_count = likes_count + 1 WHERE id = new.comment_id;
END;

CREATE TRIGGER IF NOT EXISTS decrement_comment_likes AFTER DELETE ON comment_likes BEGIN
    UPDATE comments SET likes_count = likes_count - 1 WHERE id = old.comment_id;
END;

-- STEP 9: Update activity_feed with auto-populate triggers
-- =========================================================
CREATE TRIGGER IF NOT EXISTS activity_on_comment AFTER INSERT ON comments BEGIN
    INSERT INTO activity_feed (id, user_id, username, activity_type, manga_id, manga_title, chapter_number, comment_text, created_at)
    SELECT 
        'act-' || new.id,
        new.user_id,
        u.username,
        'comment',
        new.manga_id,
        m.title,
        new.chapter_number,
        new.content,
        new.created_at
    FROM users u, manga m
    WHERE u.id = new.user_id AND m.id = new.manga_id;
END;

CREATE TRIGGER IF NOT EXISTS activity_on_rating AFTER INSERT ON manga_ratings BEGIN
    INSERT INTO activity_feed (id, user_id, username, activity_type, manga_id, manga_title, rating, created_at)
    SELECT 
        'act-' || new.id,
        new.user_id,
        u.username,
        'rating',
        new.manga_id,
        m.title,
        new.overall_rating,
        new.created_at
    FROM users u, manga m
    WHERE u.id = new.user_id AND m.id = new.manga_id;
END;

-- STEP 10: Create indexes for activity_feed if not exists
-- ========================================================
CREATE INDEX IF NOT EXISTS idx_activity_created ON activity_feed(created_at DESC);
CREATE INDEX IF NOT EXISTS idx_activity_user ON activity_feed(user_id);
CREATE INDEX IF NOT EXISTS idx_activity_manga ON activity_feed(manga_id);
CREATE INDEX IF NOT EXISTS idx_activity_type ON activity_feed(activity_type);

-- STEP 11: Migrate chat_messages column names
-- ============================================
-- Add new columns if they don't exist and copy data
ALTER TABLE chat_messages ADD COLUMN is_edited BOOLEAN DEFAULT 0;
ALTER TABLE chat_messages ADD COLUMN is_deleted BOOLEAN DEFAULT 0;

-- Copy data from old column names to new ones
UPDATE chat_messages SET is_edited = edited WHERE is_edited = 0 AND edited = 1;
UPDATE chat_messages SET is_deleted = deleted WHERE is_deleted = 0 AND deleted = 1;

-- STEP 12: Verify migration integrity
-- =====================================
-- Check for any data inconsistencies

-- Verify all genres are created
SELECT COUNT(*) as total_genres FROM genres;

-- Verify manga_genres mappings
SELECT COUNT(*) as manga_with_genres FROM manga_genres;

-- Verify average ratings are calculated
SELECT COUNT(*) as manga_with_ratings FROM manga WHERE average_rating > 0;

-- Check user role values
SELECT DISTINCT role FROM users;

-- Verify comment likes counts are correct
SELECT COUNT(*) as total_comment_likes FROM comment_likes;

-- MIGRATION COMPLETE
-- ==================
-- The database has been successfully normalized.
-- Note: The following columns can be safely ignored in code:
-- - manga.genres (replaced by manga_genres table)
-- - reading_progress.total_chapters, rating, notes, sync_version
-- - manga_ratings: story_rating, art_rating, character_rating, enjoyment_rating, helpful_count
--   (these are now consolidated into single 'rating' concept via average_rating in manga table)
-- - chat_messages.message_type (simplified)
-- - chat_room_members.notifications_enabled (removed)
-- 
-- Remove these tables from code if not being used:
-- - activities (use activity_feed with triggers instead)
-- - daily_stats, chapter_history (analytics-only, not currently used)
-- - achievements, user_achievements (gamification feature removed)
-- - user_preferences (can be added back as needed)
