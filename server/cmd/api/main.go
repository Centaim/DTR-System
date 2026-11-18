package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"log"
	"os"
	"os/signal"
	"strings"
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
	"dtr/server/internal/retention"
	"dtr/server/internal/schedule"
	"dtr/server/internal/settings"
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
	if len(os.Args) > 1 && os.Args[1] == "create-admin" {
		if err := createAdmin(ctx, pool, os.Args[2:]); err != nil {
			log.Fatal(err)
		}
		return
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

	disk, err := storage.NewDisk(cfg.PhotoDir)
	if err != nil {
		log.Fatal(err)
	}
	var store storage.Store = disk
	if cfg.PhotoKey != nil {
		if store, err = storage.NewEncrypted(disk, cfg.PhotoKey); err != nil {
			log.Fatal(err)
		}
	} else {
		log.Print("WARNING: PHOTO_KEY is not set, so photos are stored unencrypted")
	}
	appSettings, err := settings.New(ctx, pool, map[string]int{
		settings.MinGapSeconds:     cfg.MinGapSecs,
		settings.RequestWindowDays: cfg.RequestDays,
		settings.HabitualLateDays:  cfg.HabitualLateDays,
		settings.PINMaxAttempts:    cfg.PINMaxAttempts,
		settings.PINLockMinutes:    cfg.PINLockMinutes,
	})
	if err != nil {
		log.Fatal(err)
	}
	go retention.New(pool, store, appSettings).Loop(ctx)

	app := fiber.New(fiber.Config{
		// Behind a proxy, rate limits must count real clients, not the proxy.
		// The forwarded address is believed only from the listed proxies.
		EnableTrustedProxyCheck: len(cfg.TrustedProxies) > 0,
		TrustedProxies:          cfg.TrustedProxies,
		ProxyHeader:             proxyHeader(cfg.TrustedProxies),
		BodyLimit:               (cfg.MaxPhotoMB + 1) << 20,
		ReadTimeout:             15 * time.Second,
		WriteTimeout:            15 * time.Second,
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

	attSvc := attendance.NewService(pool, store, appSettings)
	schedStore := schedule.NewStore(pool)
	reqStore := requests.NewStore(pool, appSettings, loc)
	holStore := holidays.NewStore(pool)

	(&api.API{
		Pool:       pool,
		Attendance: attSvc,
		Schedules:  schedStore,
		Requests:   reqStore,
		PhotoAudit: photoaudit.NewStore(pool),
		Holidays:   holStore,
		Importer:   importer.NewService(pool),
		Kiosks:     kiosk.NewStore(pool),
		Settings:   appSettings,
		Reports:    reports.NewService(pool, attSvc, reqStore, schedStore, holStore, loc, appSettings),
		Photos:     store,
		JWTSecret:  cfg.JWTSecret,
		MaxPhotoMB: cfg.MaxPhotoMB,
		Loc:        loc,
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

func proxyHeader(trusted []string) string {
	if len(trusted) == 0 {
		return ""
	}
	return fiber.HeaderXForwardedFor
}

// createAdmin makes the first super admin on a fresh install. The temporary PIN
// is printed once and must be changed at first sign-in.
func createAdmin(ctx context.Context, pool *pgxpool.Pool, args []string) error {
	fs := flag.NewFlagSet("create-admin", flag.ExitOnError)
	no := fs.String("no", "", "employee number")
	name := fs.String("name", "", "full name")
	if err := fs.Parse(args); err != nil {
		return err
	}
	employeeNo, fullName := strings.ToUpper(strings.TrimSpace(*no)), strings.TrimSpace(*name)
	if employeeNo == "" || fullName == "" {
		return errors.New(`usage: create-admin -no ADMIN-1 -name "Full Name"`)
	}
	temp, err := auth.TempPIN()
	if err != nil {
		return err
	}
	hash, err := auth.HashPIN(temp)
	if err != nil {
		return err
	}
	tag, err := pool.Exec(ctx, `
		INSERT INTO employees (employee_no, full_name, role, pin_hash, must_change_pin)
		VALUES ($1, $2, 'super_admin', $3, TRUE) ON CONFLICT (employee_no) DO NOTHING`,
		employeeNo, fullName, hash)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return fmt.Errorf("employee number %s already exists", employeeNo)
	}
	fmt.Printf("Super admin %s created.\nTemporary PIN (shown once): %s\n", employeeNo, temp)
	return nil
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
