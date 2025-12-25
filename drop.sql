-- ============================================
-- CLEANUP: Remove Unwanted Tables & Indexes
-- ============================================

-- Drop unwanted tables
DROP TABLE IF EXISTS achievements;
DROP TABLE IF EXISTS user_achievements;
DROP TABLE IF EXISTS daily_stats;
DROP TABLE IF EXISTS chapter_history;
DROP TABLE IF EXISTS user_preferences;
DROP TABLE IF EXISTS activities;

-- Drop unwanted indexes
DROP INDEX IF EXISTS idx_achievements_category;
DROP INDEX IF EXISTS idx_user_achievements_user;
DROP INDEX IF EXISTS idx_user_achievements_unlocked;
DROP INDEX IF EXISTS idx_daily_stats_user;
DROP INDEX IF EXISTS idx_daily_stats_date;
DROP INDEX IF EXISTS idx_daily_stats_user_date;
DROP INDEX IF EXISTS idx_chapter_history_user;
DROP INDEX IF EXISTS idx_chapter_history_manga;
DROP INDEX IF EXISTS idx_chapter_history_read_at;
DROP INDEX IF EXISTS idx_custom_lists_public;
DROP INDEX IF EXISTS idx_activities_user;
DROP INDEX IF EXISTS idx_activities_manga;
DROP INDEX IF EXISTS idx_activities_type;
DROP INDEX IF EXISTS idx_activities_created;
DROP INDEX IF EXISTS idx_progress_user_id;
DROP INDEX IF EXISTS idx_progress_manga_id;
DROP INDEX IF EXISTS idx_ratings_overall;

-- ============================================
-- FIX: manga_ratings Table
-- ============================================

DROP TRIGGER IF EXISTS update_manga_rating_insert;
DROP TRIGGER IF EXISTS update_manga_rating_update;
DROP TRIGGER IF EXISTS update_manga_rating_delete;
DROP TRIGGER IF EXISTS activity_on_rating;
DROP TABLE IF EXISTS manga_ratings;

CREATE TABLE manga_ratings (
    id TEXT PRIMARY KEY,
    manga_id TEXT NOT NULL,
    user_id TEXT NOT NULL,
    rating INTEGER NOT NULL CHECK (rating BETWEEN 1 AND 10),
    review_text TEXT,
    is_spoiler BOOLEAN DEFAULT 0,
    created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
    updated_at DATETIME DEFAULT CURRENT_TIMESTAMP,
    FOREIGN KEY (manga_id) REFERENCES manga(id) ON DELETE CASCADE,
    FOREIGN KEY (user_id) REFERENCES users(id) ON DELETE CASCADE,
    UNIQUE(manga_id, user_id)
);

CREATE INDEX idx_ratings_manga ON manga_ratings(manga_id);
CREATE INDEX idx_ratings_user ON manga_ratings(user_id);
CREATE INDEX idx_ratings_created ON manga_ratings(created_at DESC);

CREATE TRIGGER update_manga_rating_insert AFTER INSERT ON manga_ratings BEGIN
    UPDATE manga
    SET average_rating = (SELECT AVG(rating) FROM manga_ratings WHERE manga_id = new.manga_id),
        rating_count = (SELECT COUNT(*) FROM manga_ratings WHERE manga_id = new.manga_id)
    WHERE id = new.manga_id;
END;

CREATE TRIGGER update_manga_rating_update AFTER UPDATE ON manga_ratings BEGIN
    UPDATE manga
    SET average_rating = (SELECT AVG(rating) FROM manga_ratings WHERE manga_id = new.manga_id)
    WHERE id = new.manga_id;
END;

CREATE TRIGGER update_manga_rating_delete AFTER DELETE ON manga_ratings BEGIN
    UPDATE manga
    SET average_rating = (SELECT COALESCE(AVG(rating), 0) FROM manga_ratings WHERE manga_id = old.manga_id),
        rating_count = (SELECT COUNT(*) FROM manga_ratings WHERE manga_id = old.manga_id)
    WHERE id = old.manga_id;
END;

CREATE TRIGGER activity_on_rating AFTER INSERT ON manga_ratings BEGIN
    INSERT INTO activity_feed (id, user_id, username, activity_type, manga_id, manga_title, rating, created_at)
    SELECT
        'act-' || new.id,
        new.user_id,
        u.username,
        'rating',
        new.manga_id,
        m.title,
        new.rating,
        new.created_at
    FROM users u, manga m
    WHERE u.id = new.user_id AND m.id = new.manga_id;
END;

-- ============================================
-- FIX: manga_external_ids Table
-- ============================================

DROP TABLE IF EXISTS manga_external_ids;

CREATE TABLE manga_external_ids (
    manga_id TEXT PRIMARY KEY,
    mangadex_id TEXT,
    anilist_id INTEGER,
    mal_id INTEGER,
    kitsu_id TEXT,
    primary_source TEXT DEFAULT 'mangadex',
    last_synced_at DATETIME,
    created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
    updated_at DATETIME DEFAULT CURRENT_TIMESTAMP,
    FOREIGN KEY (manga_id) REFERENCES manga(id) ON DELETE CASCADE
);

DROP INDEX IF EXISTS idx_external_ids_manga;
DROP INDEX IF EXISTS idx_external_ids_mangadex;
DROP INDEX IF EXISTS idx_external_ids_mal;
CREATE INDEX idx_external_mangadex ON manga_external_ids(mangadex_id);
CREATE INDEX idx_external_mal ON manga_external_ids(mal_id);
CREATE INDEX idx_external_anilist ON manga_external_ids(anilist_id);

-- ============================================
-- FIX: Chat Tables
-- ============================================

DROP TABLE IF EXISTS chat_messages;
DROP TABLE IF EXISTS chat_room_members;
DROP TABLE IF EXISTS chat_rooms;

CREATE TABLE chat_rooms (
    id TEXT PRIMARY KEY,
    name TEXT NOT NULL,
    room_type TEXT DEFAULT 'manga' CHECK (room_type IN ('general', 'manga')),
    manga_id TEXT,
    owner_id TEXT NOT NULL,
    description TEXT,
    is_active BOOLEAN DEFAULT 1,
    created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
    updated_at DATETIME DEFAULT CURRENT_TIMESTAMP,
    FOREIGN KEY (manga_id) REFERENCES manga(id) ON DELETE SET NULL,
    FOREIGN KEY (owner_id) REFERENCES users(id) ON DELETE CASCADE
);

CREATE TABLE chat_room_members (
    id TEXT PRIMARY KEY,
    room_id TEXT NOT NULL,
    user_id TEXT NOT NULL,
    role TEXT DEFAULT 'member' CHECK (role IN ('owner', 'moderator', 'member')),
    joined_at DATETIME DEFAULT CURRENT_TIMESTAMP,
    last_read_at DATETIME DEFAULT CURRENT_TIMESTAMP,
    FOREIGN KEY (room_id) REFERENCES chat_rooms(id) ON DELETE CASCADE,
    FOREIGN KEY (user_id) REFERENCES users(id) ON DELETE CASCADE,
    UNIQUE(room_id, user_id)
);

CREATE TABLE chat_messages (
    id TEXT PRIMARY KEY,
    room_id TEXT NOT NULL,
    user_id TEXT NOT NULL,
    content TEXT NOT NULL,
    reply_to_id TEXT,
    is_edited BOOLEAN DEFAULT 0,
    is_deleted BOOLEAN DEFAULT 0,
    created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
    updated_at DATETIME DEFAULT CURRENT_TIMESTAMP,
    FOREIGN KEY (room_id) REFERENCES chat_rooms(id) ON DELETE CASCADE,
    FOREIGN KEY (user_id) REFERENCES users(id) ON DELETE CASCADE
);

CREATE INDEX idx_chat_rooms_type ON chat_rooms(room_type);
CREATE INDEX idx_chat_rooms_manga ON chat_rooms(manga_id);
CREATE INDEX idx_room_members_room ON chat_room_members(room_id);
CREATE INDEX idx_room_members_user ON chat_room_members(user_id);
CREATE INDEX idx_chat_messages_room ON chat_messages(room_id);
CREATE INDEX idx_chat_messages_created ON chat_messages(created_at DESC);