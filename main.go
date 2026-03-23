package main

import (
	"log"
	"os"
	"os/signal"
	"syscall"

	"github.com/gofiber/contrib/websocket"
	"github.com/gofiber/fiber/v2"
	"github.com/gofiber/fiber/v2/middleware/logger"
	"github.com/joho/godotenv"
	"github.com/nestorlai1994/nesoli-create/db"
	"github.com/nestorlai1994/nesoli-create/handlers"
	"github.com/nestorlai1994/nesoli-create/markdown"
	"github.com/nestorlai1994/nesoli-create/ws"
)

const version = "0.1.0"

func main() {
	_ = godotenv.Load()

	port := os.Getenv("PORT")
	if port == "" {
		port = "3000"
	}

	dbURL := os.Getenv("DATABASE_URL")
	if dbURL == "" {
		log.Fatal("DATABASE_URL must be set")
	}

	pool := db.NewPool(dbURL)
	defer pool.Close()

	hub := ws.NewHub()
	go hub.Run()

	noteHandler := &handlers.NoteHandler{Pool: pool, Hub: hub}

	app := fiber.New(fiber.Config{
		AppName:               "nesoli-create v" + version,
		DisableStartupMessage: false,
	})

	app.Use(logger.New())

	app.Get("/health", func(c *fiber.Ctx) error {
		return c.JSON(fiber.Map{
			"status":  "ok",
			"service": "nesoli-create",
			"version": version,
		})
	})

	// Markdown render endpoint
	app.Post("/api/render", func(c *fiber.Ctx) error {
		type RenderRequest struct {
			Markdown string `json:"markdown"`
		}

		var req RenderRequest
		if err := c.BodyParser(&req); err != nil {
			return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
				"error": "invalid request body",
			})
		}

		if req.Markdown == "" {
			return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
				"error": "markdown field is required",
			})
		}

		result, err := markdown.Render([]byte(req.Markdown))
		if err != nil {
			return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
				"error": "failed to render markdown",
			})
		}

		return c.JSON(result)
	})

	// Note CRUD endpoints
	notes := app.Group("/api/notes")
	notes.Post("/", noteHandler.Create)
	notes.Get("/", noteHandler.List)
	notes.Get("/:slug", noteHandler.GetBySlug)
	notes.Put("/:slug", noteHandler.Update)
	notes.Delete("/:slug", noteHandler.Delete)

	// WebSocket endpoint — real-time event stream
	app.Use("/ws", func(c *fiber.Ctx) error {
		if websocket.IsWebSocketUpgrade(c) {
			return c.Next()
		}
		return fiber.ErrUpgradeRequired
	})
	app.Get("/ws", websocket.New(func(c *websocket.Conn) {
		client := ws.NewClient(hub, c)
		hub.Register(client)
		go client.WritePump()
		client.ReadPump()
	}))

	// Graceful shutdown
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGTERM, syscall.SIGINT)
	go func() {
		<-quit
		log.Println("shutting down nesoli-create...")
		_ = app.Shutdown()
	}()

	log.Printf("nesoli-create listening on :%s", port)
	if err := app.Listen(":" + port); err != nil {
		log.Fatalf("server error: %v", err)
	}
}
