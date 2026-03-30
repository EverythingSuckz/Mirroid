package main

import (
	"flag"
	"log/slog"
	"os"

	"mirroid/internal/ui"
)

func main() {
	debug := flag.Bool("debug", false, "Enable debug mode (opens logs panel on startup)")
	flag.Parse()

	level := slog.LevelInfo
	if *debug {
		level = slog.LevelDebug
	}

	handler := slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: level})
	slog.SetDefault(slog.New(handler))

	slog.Info("starting Mirroid", "debug", *debug)

	app, err := ui.NewApp(*debug)
	if err != nil {
		slog.Error("failed to start", "error", err)
		os.Exit(1)
	}
	app.Run()
}
