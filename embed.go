package main

import (
	"embed"
	"io/fs"
	"log"
	"net/http"

	"github.com/labstack/echo/v4"
)

//go:embed public/*
var publicFS embed.FS

// setupStaticFiles sets up the static file server with embedded files
func setupStaticFiles(e *echo.Echo) {
	// Get the subdirectory from the embedded filesystem
	fsys, err := fs.Sub(publicFS, "public")
	if err != nil {
		log.Fatalf("Failed to get public subdirectory: %v", err)
	}

	// Use the filesystem for static file serving
	fileServer := http.FileServer(http.FS(fsys))
	e.GET("/*", echo.WrapHandler(fileServer))
}
