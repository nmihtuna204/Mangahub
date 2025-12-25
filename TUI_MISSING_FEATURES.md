# TUI Missing Features & Implementation Guide

## Current Status

### ✅ What's Working
- Dashboard with trending manga
- Search manga by title  
- Browse by category (basic)
- Library view with status tabs (Reading/Planning/Completed/OnHold/Dropped)
- Activity feed display
- Statistics view with graceful fallback
- Library status changes (keys 1-5)

### ❌ What's Missing

## 1. MANGA DETAIL VIEW (`internal/tui/views/detail.go`)

**Missing Features:**
- [ ] **Chapter List Display**
  - Show list of available chapters
  - Navigate with arrow keys
  - Mark read/unread status
  
- [ ] **Add to Library Button**
  - Function exists but not wired: `addToLibrary()`
  - Needs to call `m.client.AddToLibrary(ctx, mangaID)`
  - Show confirmation message

- [ ] **Rating Submission**
  - Create rating modal/form
  - Input: 1-10 scale slider
  - Input: Review text (optional)
  - Call `m.client.SubmitRating(ctx, mangaID, rating, review)`

- [ ] **Comments Section**
  - Display list of comments
  - Show nested replies (parent_id)
  - "Post Comment" button
  - Like/unlike comments
  - Call `m.client.GetComments(ctx, mangaID, page, pageSize)`

- [ ] **Read Chapter Action**
  - Mark chapter as read
  - Update progress in library
  - Call `m.client.UpdateProgress(ctx, mangaID, chapter)`

**Current Issues:**
```go
// Lines 177-188 in detail.go have TODOs:
case "r":
    // TODO: Implement read next action
case "C":
    // TODO: Navigate to comments view  
case "R":
    // TODO: Open rating modal
```

## 2. LIBRARY VIEW (`internal/tui/views/library.go`)

**Missing Features:**
- [ ] **Progress Bar Display**
  - Visual progress bar: [████████░░░] 80%
  - Show current_chapter / total_chapters
  - Color coding by completion

- [ ] **Status Badges**  
  - Visual indicators: 📖 Reading, 📋 Planning, ✅ Completed
  - Favorite star: ⭐ for is_favorite=true

- [ ] **Quick Chapter Increment**
  - Press '+' to increment chapter
  - Press '-' to decrement chapter
  - Show confirmation

**Already Working:**
- ✅ Tab navigation (Reading/Planning/Completed/OnHold/Dropped)
- ✅ Status change with number keys (1-5)
- ✅ Toggle favorite with 'f'
- ✅ Update progress with 'u'

## 3. BROWSE VIEW (`internal/tui/views/browse.go`)

**Missing Features:**
- [ ] **Type Filter**
  - Filter: Manga / Manhwa / Manhua
  - Database field: `type` column in manga table
  - UI: Top filter bar

- [ ] **Status Filter**
  - Filter: Ongoing / Completed
  - Database field: `status` column
  - UI: Top filter bar

- [ ] **Genre Tags Display**
  - Show genre tags on cards
  - Parse JSON genres: `["Action","Adventure","Fantasy"]`
  - Clickable to filter

- [ ] **Sort Options**
  - Sort by: Rating / Title / Year / Popularity
  - Ascending / Descending toggle

**Current State:**
- ✅ Genre categories grid working
- ✅ Basic search by genre name
- ⚠️  Needs better filtering API integration

## 4. CHAT VIEW (`internal/tui/views/chat.go`)

**Missing Integration:**
- [ ] **WebSocket Connection**
  - File exists: `internal/tui/network/ws_client.go`
  - Not integrated in main `app.go`
  - Need to wire chat view to WS client

- [ ] **Room Management**
  - General chat room: `room_id='general'`
  - Per-manga rooms: `room_id='manga_{manga_id}'`
  - Auto-create rooms when joining

- [ ] **Message Send/Receive**
  - Send: `ws.SendMessage(roomID, content)`
  - Receive: Handle WebSocket messages
  - Display in chat viewport

- [ ] **User Presence**
  - Show online user count
  - Display "user is typing..." indicators

**Connection Details:**
- WebSocket endpoint: `ws://localhost:8080/ws/chat?room_id={room_id}`
- Requires JWT token in header
- Message format: see `internal/websocket/models.go`

## 5. RATING FEATURE (New Component Needed)

**Create: `internal/tui/views/rating_modal.go`**

```go
type RatingModalModel struct {
    rating      float64  // 1.0 to 10.0
    reviewText  string
    mangaID     string
    mangaTitle  string
    submitting  bool
}

// UI Elements:
// - Rating slider: ⭐ 1 2 3 4 5 6 7 8 9 10
// - Review textarea (optional)
// - [Submit] [Cancel] buttons
// - Show existing rating if user already rated
```

**API Call:**
```go
err := m.client.SubmitRating(ctx, mangaID, rating, reviewText)
```

## 6. COMMENT FEATURE (New Component Needed)

**Create: `internal/tui/views/comments_view.go`**

```go
type CommentsViewModel struct {
    mangaID     string
    comments    []models.CommentWithUser
    inputBox    textarea.Model
    selectedIdx int
    replyTo     *string  // ID of comment being replied to
}

// Features:
// - List comments with nested replies
// - Post new comment
// - Reply to existing comment  
// - Like/unlike button
// - Pagination (load more)
```

**API Calls:**
```go
// Get comments
comments, err := m.client.GetComments(ctx, mangaID, page, pageSize)

// Post comment (TODO: add to client.go)
err := m.client.PostComment(ctx, mangaID, content, parentID)

// Like comment (TODO: add to client.go)  
err := m.client.LikeComment(ctx, commentID)
```

## Implementation Steps

### Phase 1: Complete Detail View (High Priority)
1. Add "Add to Library" button functionality
2. Display chapter list (even if just numbers 1-N)
3. Wire up read chapter action
4. Show existing rating if user rated

### Phase 2: Rating & Comments (Medium Priority)
5. Create rating modal component
6. Create comments view component
7. Wire both to detail view

### Phase 3: Chat Integration (Medium Priority)
8. Integrate WebSocket client in app.go
9. Connect chat view to WS
10. Handle room joining from detail view ('c' key)

### Phase 4: Enhanced Browse (Low Priority)
11. Add type/status filters
12. Add sort options
13. Better genre tag display

## API Client Missing Methods

Add to `internal/tui/api/client.go`:

```go
// POST /manga/:id/comments
func (c *Client) PostComment(ctx context.Context, mangaID string, content string, chapterNum *int, parentID *string) error

// POST /comments/:id/like  
func (c *Client) LikeComment(ctx context.Context, commentID string) error

// DELETE /comments/:id/like
func (c *Client) UnlikeComment(ctx context.Context, commentID string) error
```

## Testing Checklist

- [ ] Can add manga to library from detail view
- [ ] Can view chapter list
- [ ] Can mark chapter as read
- [ ] Can submit rating (1-10 with review)
- [ ] Can view existing comments
- [ ] Can post new comment
- [ ] Can reply to comment
- [ ] Can like/unlike comment
- [ ] Can join chat room from manga detail
- [ ] Can send/receive chat messages
- [ ] Can filter browse by type
- [ ] Can filter browse by genre
- [ ] Library shows progress bars
- [ ] Library shows status badges

## Database Schema Verification

All required tables exist:
- ✅ `manga` (id, title, genres, type, status, rating)
- ✅ `reading_progress` (user_id, manga_id, current_chapter, status, is_favorite)
- ✅ `manga_ratings` (manga_id, user_id, overall_rating, review_text)
- ✅ `comments` (manga_id, user_id, content, chapter_number, parent_id)
- ✅ `comment_likes` (comment_id, user_id)
- ✅ `chat_rooms` (id, name, room_type, manga_id)
- ✅ `chat_messages` (room_id, user_id, content)

## Next Steps

1. **Immediate**: Complete detail view with Add to Library button
2. **Today**: Add chapter list display
3. **Tomorrow**: Create rating modal
4. **Next**: Create comments view
5. **Finally**: Integrate WebSocket chat

---

**Priority Order:**
1. Add to Library (5 min fix)
2. Chapter list (30 min)
3. Rating modal (2 hours)
4. Comments view (3 hours)
5. Chat WebSocket (2 hours)
6. Browse filters (1 hour)
