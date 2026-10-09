package main

import (
	"context"
	"database/sql"
	"embed"
	"github.com/wailsapp/wails/v3/pkg/application"
	"log"
	_ "modernc.org/sqlite"
	"os"
	"strings"
	"time"
	"vault-wails/internal/db"
)

//go:embed all:frontend/dist
var assets embed.FS

//go:embed wordlist.txt
var largeWordlist string

//go:embed internal/db/schema.sql
var ddl string

type services struct {
	dbQ        *db.Queries
	wordlist   []string
	dbPath     string
	isFirstRun bool
}

func init() {
	// Register a custom event whose associated data type is string.
	// This is not required, but the binding generator will pick up registered events
	// and provide a strongly typed JS/TS API for them.
	application.RegisterEvent[string]("time")
}

func main() {
	dbPath := "./vault.db"
	_, err := os.Stat(dbPath)
	isFirstRun := os.IsNotExist(err)

	conn, err := sql.Open("sqlite", "file:vault.db?_journal_mode=WAL&_busy_timeout=5000")
	if err != nil {
		log.Fatalf("error opening database: %v", err)
	}
	defer conn.Close()

	if err := conn.Ping(); err != nil {
		log.Fatalf("error pinging database: %v", err)
	}
	conn.SetMaxOpenConns(1)

	ctx := context.Background()
	if _, err := conn.ExecContext(ctx, ddl); err != nil {
		log.Fatalf("failed to init database schema: %v", err)
	}
	dbQueries := db.New(conn)

	wordlist := strings.Split(largeWordlist, "\n")

	vaultService := NewVaultService(dbQueries, wordlist, dbPath, isFirstRun)

	app := application.New(application.Options{
		Name:        "vault-wails",
		Description: "Password Manager in Go with Sqlite and Wails",
		Services: []application.Service{
			application.NewService(vaultService),
		},
		Assets: application.AssetOptions{
			Handler: application.AssetFileServerFS(assets),
		},
		Mac: application.MacOptions{
			ApplicationShouldTerminateAfterLastWindowClosed: true,
		},
	})

	app.Window.NewWithOptions(application.WebviewWindowOptions{
		Title:  "vaultOrb",
		Width:  1000,
		Height: 618,
		Mac: application.MacWindow{
			InvisibleTitleBarHeight: 50,
			Backdrop:                application.MacBackdropTranslucent,
			TitleBar:                application.MacTitleBarHiddenInset,
		},
		BackgroundColour: application.NewRGB(6, 7, 15),
		URL:              "/",
	})

	go func() {
		for {
			now := time.Now().Format(time.RFC1123)
			app.Event.Emit("time", now)
			time.Sleep(time.Second)
		}
	}()

	err = app.Run()

	if err != nil {
		log.Fatal(err)
	}
}
