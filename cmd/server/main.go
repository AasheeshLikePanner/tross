package main

import (
	"log/slog"
	"net/http"
	"os"

	"github.com/tross/linkedin-profile-api/internal/api"
	"github.com/tross/linkedin-profile-api/internal/linkedin"
)

func main() {
	liAt := os.Getenv("LINKEDIN_LI_AT")
	jsessionID := os.Getenv("LINKEDIN_JSESSIONID")

	if liAt == "" || jsessionID == "" {
		slog.Error("LINKEDIN_LI_AT and LINKEDIN_JSESSIONID must be set")
		os.Exit(1)
	}

	bcookie := os.Getenv("LINKEDIN_BCOOKIE")
	bscookie := os.Getenv("LINKEDIN_BSCOOKIE")
	lidc := os.Getenv("LINKEDIN_LIDC")

	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}

	client, err := linkedin.NewClient(liAt, jsessionID, bcookie, bscookie, lidc)
	if err != nil {
		slog.Error("failed to initialize linkedin client", "err", err)
		os.Exit(1)
	}

	handler := api.NewHandler(client)

	slog.Info("server starting", "port", port)
	if err := http.ListenAndServe(":"+port, handler.Routes()); err != nil {
		slog.Error("server error", "err", err)
		os.Exit(1)
	}
}
