package main

import (
	"log/slog"
	"net/http"
	"os"

	"gitlab.home.firblab.org/applications/jobhunt-os/internal/server"
)

func main() {
	addr := os.Getenv("JOBHUNT_ADDR")
	if addr == "" {
		addr = "127.0.0.1:8080"
	}

	handler := server.New()

	slog.Info("starting jobhunt-os", "addr", addr)
	if err := http.ListenAndServe(addr, handler); err != nil {
		slog.Error("server stopped", "error", err)
		os.Exit(1)
	}
}
