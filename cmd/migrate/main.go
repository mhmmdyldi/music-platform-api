// Command migrate applies or inspects the database schema migrations embedded
// from db/migrations.
//
//	migrate up        apply every pending migration
//	migrate status    list migrations and whether each is applied
//	migrate down      roll back the most recent migration
//
// It reads the same configuration as the API (APP_ENV, DATABASE_URL, ...).
// In deployments it runs once per release, before the new API version starts.
// `down` is refused in production unless -allow-production-down is passed.
package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"os"
	"os/signal"
	"syscall"

	"github.com/mhmmdyldi/music-platform-api/db/migrations"
	"github.com/mhmmdyldi/music-platform-api/internal/config"
	"github.com/mhmmdyldi/music-platform-api/internal/platform/database"
)

func main() {
	if err := run(os.Args[1:]); err != nil {
		fmt.Fprintln(os.Stderr, "migrate:", err)
		os.Exit(1)
	}
}

func run(args []string) error {
	fs := flag.NewFlagSet("migrate", flag.ContinueOnError)
	allowProductionDown := fs.Bool("allow-production-down", false, "permit `down` when APP_ENV=production")
	fs.Usage = func() {
		fmt.Fprintln(fs.Output(), "usage: migrate [-allow-production-down] up|down|status")
		fs.PrintDefaults()
	}
	if err := fs.Parse(args); err != nil {
		return err
	}
	if fs.NArg() != 1 {
		fs.Usage()
		return errors.New("exactly one command is required")
	}
	command := fs.Arg(0)

	cfg, err := config.Load(os.LookupEnv)
	if err != nil {
		return err
	}
	if command == "down" && cfg.Env == config.EnvProduction && !*allowProductionDown {
		return errors.New("refusing to roll back production; pass -allow-production-down if this is intended")
	}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	pool, err := database.Open(ctx, database.PoolOptions{
		URL:            cfg.Database.URL,
		MaxConns:       2,
		ConnectTimeout: cfg.Database.ConnectTimeout,
	})
	if err != nil {
		return err
	}
	defer pool.Close()

	m, err := database.NewMigrator(pool, migrations.FS)
	if err != nil {
		return err
	}
	defer m.Close()

	fmt.Printf("env=%s database=%s\n", cfg.Env, cfg.Database.Name())

	switch command {
	case "up":
		applied, err := m.Up(ctx)
		if err != nil {
			return err
		}
		if len(applied) == 0 {
			fmt.Println("schema is up to date")
		}
		for _, v := range applied {
			fmt.Printf("applied %05d\n", v)
		}
	case "down":
		v, err := m.DownOne(ctx)
		if err != nil {
			return err
		}
		fmt.Printf("rolled back %05d\n", v)
	case "status":
		statuses, err := m.Status(ctx)
		if err != nil {
			return err
		}
		for _, s := range statuses {
			state := "pending"
			if s.Applied {
				state = "applied"
			}
			fmt.Printf("%05d  %-8s %s\n", s.Version, state, s.File)
		}
	default:
		fs.Usage()
		return fmt.Errorf("unknown command %q", command)
	}
	return nil
}
