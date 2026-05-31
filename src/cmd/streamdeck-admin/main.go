package main

import (
	"fmt"
	"net/http"
	"os"

	"streamdeck-go/internal/admin"
	"streamdeck-go/internal/app"
)

func main() {
	configPath, err := app.DefaultConfigPath()
	if err != nil {
		fatal(err)
	}
	server := admin.New(admin.Options{
		ConfigPath: configPath,
		Addr:       envDefault("STREAMDECK_ADMIN_ADDR", "127.0.0.1:8787"),
	})
	fmt.Println("streamdeck-admin:", "http://"+server.Addr())
	if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		fatal(err)
	}
}

func envDefault(name, fallback string) string {
	if value := os.Getenv(name); value != "" {
		return value
	}
	return fallback
}

func fatal(err error) {
	fmt.Fprintln(os.Stderr, err)
	os.Exit(1)
}
