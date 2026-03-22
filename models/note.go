package models

import (
	"regexp"
	"strings"
	"time"

	"github.com/google/uuid"
)

type Note struct {
	ID          uuid.UUID  `json:"id"`
	AuthorID    uuid.UUID  `json:"author_id"`
	Title       string     `json:"title"`
	Slug        string     `json:"slug"`
	Content     string     `json:"content"`
	Excerpt     *string    `json:"excerpt"`
	Tags        []string   `json:"tags"`
	Category    *string    `json:"category"`
	Published   bool       `json:"published"`
	PublishedAt *time.Time `json:"published_at"`
	CreatedAt   time.Time  `json:"created_at"`
	UpdatedAt   time.Time  `json:"updated_at"`
}

type NoteWithHTML struct {
	Note
	HTML        string                 `json:"html"`
	Frontmatter map[string]interface{} `json:"frontmatter"`
}

type CreateNoteRequest struct {
	Title    string   `json:"title"`
	Content  string   `json:"content"`
	Tags     []string `json:"tags"`
	Category *string  `json:"category"`
}

type UpdateNoteRequest struct {
	Title    *string  `json:"title"`
	Content  *string  `json:"content"`
	Tags     []string `json:"tags"`
	Category *string  `json:"category"`
}

type NoteListResponse struct {
	Notes   []Note `json:"notes"`
	Total   int64  `json:"total"`
	Page    int    `json:"page"`
	PerPage int    `json:"per_page"`
}

var (
	nonAlphanumRe = regexp.MustCompile(`[^a-z0-9]+`)
	dashTrimRe    = regexp.MustCompile(`^-+|-+$`)
)

// Slugify converts a title into a URL-safe slug.
func Slugify(title string) string {
	s := strings.ToLower(strings.TrimSpace(title))
	s = nonAlphanumRe.ReplaceAllString(s, "-")
	s = dashTrimRe.ReplaceAllString(s, "")
	if s == "" {
		s = "untitled"
	}
	return s
}

// Excerpt extracts the first ~200 characters from plain text content.
func Excerpt(content string, maxLen int) string {
	if maxLen <= 0 {
		maxLen = 200
	}
	// Strip markdown formatting for a cleaner excerpt
	text := strings.TrimSpace(content)
	text = regexp.MustCompile(`[#*_~\[\]()>` + "`" + `]`).ReplaceAllString(text, "")
	text = strings.Join(strings.Fields(text), " ")
	if len(text) > maxLen {
		text = text[:maxLen] + "…"
	}
	return text
}
