package logger

import (
	"log/slog"
	"os"

	"github.com/lmittmann/tint"
)

var LogLevel slog.LevelVar

func init() {
	lg := slog.New(tint.NewHandler(os.Stderr, &tint.Options{
		AddSource:  true,
		Level:      &LogLevel,
		TimeFormat: "2006 Jan 02 15:04:05",
	}))
	slog.SetDefault(lg)
}
