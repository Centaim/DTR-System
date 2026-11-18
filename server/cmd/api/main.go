package main

import (
	"context"
	"errors"
	"log"
	"os"
	"os/signal"
	"syscall"
	"time"

	"dtr/server/internal/api"
	"dtr/server/internal/attendance"
	"dtr/server/internal/auth"
	"dtr/server/internal/config"
	"dtr/server/internal/db"
	"dtr/server/internal/storage"
	"dtr/server/migrations"

	"github.com/gofiber/fiber/v2"
	"github.com/gofiber/fiber/v2/middleware/cors"
	"github.com/gofiber/fiber/v2/middleware/recover"
	"github.com/jackc/pgx/v5/pgxpool"
)

func main() {
	cfg, err := config.Load()
	if err != nil {
		log.Fatal(err)
	}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	pool, err := db.Open(ctx, cfg.DatabaseURL)
	if err != nil {
		log.Fatal(err)
	}
	defer pool.Close()

	if err := db.Migrate(ctx, pool, migrations.FS); err != nil {
		log.Fatal(err)
	}
	if cfg.SeedDemo {
		if err := seedDemo(ctx, pool); err != nil {
			log.Fatal(err)
		}
	}

	store, err := storage.NewDisk(cfg.PhotoDir)
	if err != nil {
		log.Fatal(err)
	}

	app := fiber.New(fiber.Config{
		BodyLimit:    (cfg.MaxPhotoMB + 1) << 20,
		ReadTimeout:  15 * time.Second,
		WriteTimeout: 15 * time.Second,
		ErrorHandler: func(c *fiber.Ctx, err error) error {
			var fe *fiber.Error
			if errors.As(err, &fe) {
				return c.Status(fe.Code).JSON(fiber.Map{"error": fiber.Map{"code": "error", "message": fe.Message}})
			}
			log.Printf("error: %s %s: %v", c.Method(), c.Path(), err) // never log request bodies or photos
			return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
				"error": fiber.Map{"code": "internal", "message": "Something went wrong. Try again."},
			})
		},
	})
	app.Use(recover.New())
	app.Use(cors.New(cors.Config{
		AllowOrigins: cfg.CORSOrigins,
		AllowHeaders: "Authorization, Content-Type",
		AllowMethods: "GET, POST, OPTIONS",
	}))

	(&api.API{
		Pool:       pool,
		Attendance: attendance.NewService(pool, store, time.Duration(cfg.MinGapSecs)*time.Second),
		JWTSecret:  cfg.JWTSecret,
		MaxPhotoMB: cfg.MaxPhotoMB,
	}).Routes(app)

	go func() {
		<-ctx.Done()
		_ = app.ShutdownWithTimeout(10 * time.Second)
	}()
	log.Printf("listening on %s", cfg.Addr)
	if err := app.Listen(cfg.Addr); err != nil {
		log.Fatal(err)
	}
}

// seedDemo inserts a synthetic employee for local demos only. Fake data, never real.
func seedDemo(ctx context.Context, pool *pgxpool.Pool) error {
	hash, err := auth.HashPIN("123456")
	if err != nil {
		return err
	}
	_, err = pool.Exec(ctx, `
		INSERT INTO employees (employee_no, full_name, position, office, pin_hash)
		VALUES ('DEMO-0001', 'Demo Employee (synthetic)', 'Administrative Aide', 'Records Section', $1)
		ON CONFLICT (employee_no) DO NOTHING`, hash)
	return err
}
