package main

import (
	"context"
	"fmt"
	"log"
	"os"

	"slackcheers/internal/config"
	"slackcheers/internal/database"
)

func main() {
	cmd := "up"
	if len(os.Args) > 1 {
		cmd = os.Args[1]
	}

	cfg, err := config.Load()
	if err != nil {
		log.Fatalf("load config: %v", err)
	}

	ctx := context.Background()
	db, err := database.OpenPostgres(ctx, cfg.DB)
	if err != nil {
		log.Fatalf("connect db: %v", err)
	}
	defer db.Close()

	switch cmd {
	case "up":
		err = database.UpMigrations(ctx, db, cfg.DB.MigrationsDir)
	case "down":
		err = database.DownOneMigration(ctx, db, cfg.DB.MigrationsDir)
	case "status":
		status, statusErr := database.MigrationStatus(ctx, db, cfg.DB.MigrationsDir)
		if statusErr == nil {
			fmt.Println(status)
		}
		err = statusErr
	default:
		log.Fatalf("unsupported command %q (use up|down|status)", cmd)
	}

	if err != nil {
		log.Fatalf("migration command failed: %v", err)
	}

	fmt.Printf("migration command %q completed\n", cmd)
}
