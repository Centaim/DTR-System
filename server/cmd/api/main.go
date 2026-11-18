package main

import (
	"context"
	"errors"
	"log"
	"os"
	"os/signal"
	"syscall"
	"time"
	_ "time/tzdata" // Asia/Manila must resolve on minimal containers

	"dtr/server/internal/api"
	"dtr/server/internal/attendance"
	"dtr/server/internal/auth"
	"dtr/server/internal/config"
	"dtr/server/internal/db"
	"dtr/server/internal/holidays"
	"dtr/server/internal/importer"
	"dtr/server/internal/kiosk"
	"dtr/server/internal/photoaudit"
	"dtr/server/internal/reports"
	"dtr/server/internal/requests"
	"dtr/server/internal/schedule"
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

	loc, err := time.LoadLocation("Asia/Manila")
	if err != nil {
		log.Fatal(err)
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

	attSvc := attendance.NewService(pool, store, time.Duration(cfg.MinGapSecs)*time.Second)
	schedStore := schedule.NewStore(pool)
	reqStore := requests.NewStore(pool, cfg.RequestDays, loc)
	holStore := holidays.NewStore(pool)

	(&api.API{
		Pool:           pool,
		Attendance:     attSvc,
		Schedules:      schedStore,
		Requests:       reqStore,
		PhotoAudit:     photoaudit.NewStore(pool),
		Holidays:       holStore,
		Importer:       importer.NewService(pool),
		Kiosks:         kiosk.NewStore(pool),
		PINMaxAttempts: cfg.PINMaxAttempts,
		PINLockMinutes: cfg.PINLockMinutes,
		Reports:        reports.NewService(pool, attSvc, reqStore, schedStore, holStore, loc, cfg.HabitualLateDays),
		Photos:         store,
		JWTSecret:      cfg.JWTSecret,
		MaxPhotoMB:     cfg.MaxPhotoMB,
		Loc:            loc,
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

// seedDemo inserts synthetic accounts for local demos only. Fake data, never real.
func seedDemo(ctx context.Context, pool *pgxpool.Pool) error {
	hash, err := auth.HashPIN("123456")
	if err != nil {
		return err
	}
	for _, e := range []struct{ no, name, pos, office, role string }{
		{"DEMO-0001", "Demo Employee (synthetic)", "Administrative Aide", "Records Section", "employee"},
		{"HR-0001", "Demo HR Admin (synthetic)", "Administrative Officer", "Human Resources", "hr_admin"},
		{"ADMIN-0001", "Demo Super Admin (synthetic)", "Systems Administrator", "Information Technology", "super_admin"},
	} {
		if _, err := pool.Exec(ctx, `
			INSERT INTO employees (employee_no, full_name, position, office, role, pin_hash)
			VALUES ($1, $2, $3, $4, $5::employee_role, $6)
			ON CONFLICT (employee_no) DO NOTHING`, e.no, e.name, e.pos, e.office, e.role, hash); err != nil {
			return err
		}
	}
	return nil
}
