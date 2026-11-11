package main

import (
	"context"
	"dota-helper/services"

	"embed"
	"log"
	"net/http"
	"os"
	"time"

	"github.com/joho/godotenv"
	"github.com/wailsapp/wails/v3/pkg/application"
)

var assets embed.FS

type stratzTransport struct {
	Token string
	Base  http.RoundTripper
}

func (t *stratzTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	// STRATZ requires both the Bearer prefix and a User-Agent
	req.Header.Set("Authorization", "Bearer "+t.Token)
	req.Header.Set("User-Agent", "STRATZ_API")

	// Ensure Base is set, fallback to default if not
	base := t.Base
	if base == nil {
		base = http.DefaultTransport
	}
	return base.RoundTrip(req)
}

func main() {

	var db = InitDB()
	sqlDB, err := db.DB()
	if err == nil {
		defer sqlDB.Close()
	}

	if err := godotenv.Load(); err != nil {
		log.Println("No .env file found or error reading it (ignoring for production)")
	}

	stratzToken := os.Getenv("STRATZ_TOKEN")

	httpTimeout, err := time.ParseDuration(os.Getenv("HTTP_TIMEOUT"))
	if err != nil {
		httpTimeout = time.Minute
	}

	openDotaHttp := &http.Client{
		Timeout: httpTimeout,
	}

	stratzHttp := &http.Client{
		Timeout: httpTimeout,
		Transport: &stratzTransport{
			Token: stratzToken, // Uses the token from godotenv
		},
	}

	iconHttp := &http.Client{
		Timeout: httpTimeout,
		CheckRedirect: func(req *http.Request, via []*http.Request) error {
			// http.ErrUseLastResponse stops redirects exactly like AllowAutoRedirect = false
			return http.ErrUseLastResponse
		},
	}
	openDotaClient := &services.OpenDotaClient{Http: openDotaHttp}
	stratzClient := &services.StratzClient{Http: stratzHttp, Token: stratzToken}

	heroImporter := &services.HeroImporter{
		DB:       db,
		OpenDota: openDotaClient,
	}
	metaCache := services.NewMetaCache(db)

	metaImporter := &services.MetaImporter{
		DB:        db,
		Stratz:    stratzClient,
		Heroes:    heroImporter,
		MetaCache: metaCache,
	}

	// Core application services (Controllers)
	playersService := &services.Players{DB: db}

	log.Println("Syncing heroes from OpenDota...")
	if err := heroImporter.SyncAsync(context.Background()); err != nil {
		log.Printf("Warning: Initial hero sync failed: %v", err)
	}

	app := application.New(application.Options{
		Name:        "dota-helper",
		Description: "A Dota 2 helper application.",
		Services: []application.Service{
			application.NewService(&services.Heroes{DB: db, IconClient: iconHttp}),
			application.NewService(&services.Players{DB: db, Stratz: stratzClient}),
			application.NewService(&services.Recommendations{Players: playersService, MetaCache: metaCache}),
			application.NewService(&services.Meta{DB: db, MetaImporter: metaImporter, Token: stratzToken}),
		},
		Assets: application.AssetOptions{
			Handler: application.AssetFileServerFS(assets),
		},
		Mac: application.MacOptions{
			ApplicationShouldTerminateAfterLastWindowClosed: true,
		},
	})

	app.Window.NewWithOptions(application.WebviewWindowOptions{
		Title: "Dota Helper",

		// // Window sized to the golden ratio (1000 / 618 ≈ 1.618).
		// Width:  1000,
		// Height: 618,
		Mac: application.MacWindow{
			InvisibleTitleBarHeight: 50,
			Backdrop:                application.MacBackdropTranslucent,
			TitleBar:                application.MacTitleBarHiddenInset,
		},
		BackgroundColour: application.NewRGBA(62, 63, 69, 255),
		URL:              "/",
	})

	if err := app.Run(); err != nil {
		log.Fatal(err)
	}
}
