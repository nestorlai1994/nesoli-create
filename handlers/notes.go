package handlers

import (
	"context"
	"strconv"
	"time"

	"github.com/gofiber/fiber/v2"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/nestorlai1994/nesoli-create/markdown"
	"github.com/nestorlai1994/nesoli-create/models"
	"github.com/nestorlai1994/nesoli-create/ws"
)

const systemUserID = "00000000-0000-0000-0000-000000000001"

type NoteHandler struct {
	Pool *pgxpool.Pool
	Hub  *ws.Hub
}

func (h *NoteHandler) Create(c *fiber.Ctx) error {
	var req models.CreateNoteRequest
	if err := c.BodyParser(&req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "invalid request body"})
	}
	if req.Title == "" {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "title is required"})
	}

	slug := models.Slugify(req.Title)
	excerpt := models.Excerpt(req.Content, 200)

	ctx, cancel := context.WithTimeout(c.Context(), 5*time.Second)
	defer cancel()

	// Deduplicate slug by appending a suffix if needed
	baseSlug := slug
	for i := 2; ; i++ {
		var exists bool
		err := h.Pool.QueryRow(ctx, "SELECT EXISTS(SELECT 1 FROM notes WHERE slug = $1)", slug).Scan(&exists)
		if err != nil {
			return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "database error"})
		}
		if !exists {
			break
		}
		slug = baseSlug + "-" + strconv.Itoa(i)
	}

	tags := req.Tags
	if tags == nil {
		tags = []string{}
	}

	var note models.Note
	err := h.Pool.QueryRow(ctx,
		`INSERT INTO notes (author_id, title, slug, content, excerpt, tags, category)
		 VALUES ($1, $2, $3, $4, $5, $6, $7)
		 RETURNING id, author_id, title, slug, content, excerpt, tags, category, published, published_at, created_at, updated_at`,
		systemUserID, req.Title, slug, req.Content, excerpt, tags, req.Category,
	).Scan(
		&note.ID, &note.AuthorID, &note.Title, &note.Slug, &note.Content,
		&note.Excerpt, &note.Tags, &note.Category, &note.Published, &note.PublishedAt,
		&note.CreatedAt, &note.UpdatedAt,
	)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "failed to create note: " + err.Error()})
	}

	h.Hub.BroadcastEvent("note.created", note)
	return c.Status(fiber.StatusCreated).JSON(note)
}

func (h *NoteHandler) List(c *fiber.Ctx) error {
	page, _ := strconv.Atoi(c.Query("page", "1"))
	perPage, _ := strconv.Atoi(c.Query("per_page", "20"))
	if page < 1 {
		page = 1
	}
	if perPage < 1 || perPage > 100 {
		perPage = 20
	}
	offset := (page - 1) * perPage

	tag := c.Query("tag")
	published := c.Query("published")

	ctx, cancel := context.WithTimeout(c.Context(), 5*time.Second)
	defer cancel()

	// Build dynamic query
	query := "SELECT id, author_id, title, slug, content, excerpt, tags, category, published, published_at, created_at, updated_at FROM notes WHERE 1=1"
	countQuery := "SELECT COUNT(*) FROM notes WHERE 1=1"
	args := []interface{}{}
	argIdx := 1

	if tag != "" {
		filter := " AND $" + strconv.Itoa(argIdx) + " = ANY(tags)"
		query += filter
		countQuery += filter
		args = append(args, tag)
		argIdx++
	}
	if published == "true" {
		filter := " AND published = true"
		query += filter
		countQuery += filter
	} else if published == "false" {
		filter := " AND published = false"
		query += filter
		countQuery += filter
	}

	var total int64
	countArgs := make([]interface{}, len(args))
	copy(countArgs, args)
	if err := h.Pool.QueryRow(ctx, countQuery, countArgs...).Scan(&total); err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "count failed"})
	}

	query += " ORDER BY created_at DESC LIMIT $" + strconv.Itoa(argIdx) + " OFFSET $" + strconv.Itoa(argIdx+1)
	args = append(args, perPage, offset)

	rows, err := h.Pool.Query(ctx, query, args...)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "query failed"})
	}
	defer rows.Close()

	notes := []models.Note{}
	for rows.Next() {
		var n models.Note
		if err := rows.Scan(
			&n.ID, &n.AuthorID, &n.Title, &n.Slug, &n.Content,
			&n.Excerpt, &n.Tags, &n.Category, &n.Published, &n.PublishedAt,
			&n.CreatedAt, &n.UpdatedAt,
		); err != nil {
			return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "scan failed"})
		}
		notes = append(notes, n)
	}

	return c.JSON(models.NoteListResponse{
		Notes:   notes,
		Total:   total,
		Page:    page,
		PerPage: perPage,
	})
}

func (h *NoteHandler) GetBySlug(c *fiber.Ctx) error {
	slug := c.Params("slug")

	ctx, cancel := context.WithTimeout(c.Context(), 5*time.Second)
	defer cancel()

	var note models.Note
	err := h.Pool.QueryRow(ctx,
		`SELECT id, author_id, title, slug, content, excerpt, tags, category, published, published_at, created_at, updated_at
		 FROM notes WHERE slug = $1`, slug,
	).Scan(
		&note.ID, &note.AuthorID, &note.Title, &note.Slug, &note.Content,
		&note.Excerpt, &note.Tags, &note.Category, &note.Published, &note.PublishedAt,
		&note.CreatedAt, &note.UpdatedAt,
	)
	if err != nil {
		return c.Status(fiber.StatusNotFound).JSON(fiber.Map{"error": "note not found"})
	}

	// Render markdown to HTML
	rendered, renderErr := markdown.Render([]byte(note.Content))
	result := models.NoteWithHTML{Note: note}
	if renderErr == nil {
		result.HTML = rendered.HTML
		result.Frontmatter = rendered.Frontmatter
	}

	return c.JSON(result)
}

func (h *NoteHandler) Update(c *fiber.Ctx) error {
	slug := c.Params("slug")

	var req models.UpdateNoteRequest
	if err := c.BodyParser(&req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "invalid request body"})
	}

	ctx, cancel := context.WithTimeout(c.Context(), 5*time.Second)
	defer cancel()

	// Fetch current note
	var note models.Note
	err := h.Pool.QueryRow(ctx,
		`SELECT id, author_id, title, slug, content, excerpt, tags, category, published, published_at, created_at, updated_at
		 FROM notes WHERE slug = $1`, slug,
	).Scan(
		&note.ID, &note.AuthorID, &note.Title, &note.Slug, &note.Content,
		&note.Excerpt, &note.Tags, &note.Category, &note.Published, &note.PublishedAt,
		&note.CreatedAt, &note.UpdatedAt,
	)
	if err != nil {
		return c.Status(fiber.StatusNotFound).JSON(fiber.Map{"error": "note not found"})
	}

	// Apply partial updates
	if req.Title != nil {
		note.Title = *req.Title
		note.Slug = models.Slugify(*req.Title)
	}
	if req.Content != nil {
		note.Content = *req.Content
		exc := models.Excerpt(*req.Content, 200)
		note.Excerpt = &exc
	}
	if req.Tags != nil {
		note.Tags = req.Tags
	}
	if req.Category != nil {
		note.Category = req.Category
	}

	err = h.Pool.QueryRow(ctx,
		`UPDATE notes SET title = $1, slug = $2, content = $3, excerpt = $4, tags = $5, category = $6, updated_at = NOW()
		 WHERE id = $7
		 RETURNING id, author_id, title, slug, content, excerpt, tags, category, published, published_at, created_at, updated_at`,
		note.Title, note.Slug, note.Content, note.Excerpt, note.Tags, note.Category, note.ID,
	).Scan(
		&note.ID, &note.AuthorID, &note.Title, &note.Slug, &note.Content,
		&note.Excerpt, &note.Tags, &note.Category, &note.Published, &note.PublishedAt,
		&note.CreatedAt, &note.UpdatedAt,
	)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "update failed: " + err.Error()})
	}

	h.Hub.BroadcastEvent("note.updated", note)
	return c.JSON(note)
}

func (h *NoteHandler) Delete(c *fiber.Ctx) error {
	slug := c.Params("slug")

	ctx, cancel := context.WithTimeout(c.Context(), 5*time.Second)
	defer cancel()

	tag, err := h.Pool.Exec(ctx, "DELETE FROM notes WHERE slug = $1", slug)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "delete failed"})
	}
	if tag.RowsAffected() == 0 {
		return c.Status(fiber.StatusNotFound).JSON(fiber.Map{"error": "note not found"})
	}

	h.Hub.BroadcastEvent("note.deleted", fiber.Map{"slug": slug})
	return c.SendStatus(fiber.StatusNoContent)
}
