package logger

import (
	"github.com/mattn/go-isatty"
	"github.com/phsym/console-slog"
	slogformatter "github.com/samber/slog-formatter"
	"io"
	"log/slog"
	"os"
	"strings"
	"time"
)

func GetLogLevel(in string) slog.Level {
	in = strings.ToUpper(in)
	switch in {
	case "DEBUG":
		return slog.LevelDebug
	case "WARN":
		return slog.LevelWarn
	case "ERROR", "ERR":
		return slog.LevelError
	default:
		return slog.LevelInfo
	}
}

func GetDefault(level slog.Level) (*slog.Logger, error) {

	useTty := isatty.IsTerminal(os.Stdout.Fd()) || isatty.IsCygwinTerminal(os.Stdout.Fd())
	//useTty = false

	var defaultHandler slog.Handler
	if useTty {
		consoleHan := console.NewHandler(os.Stdout, &console.HandlerOptions{
			Level: level,
			//AddSource:  true,
			TimeFormat: time.Kitchen,
		})

		// formatters
		var fmts []slogformatter.Formatter
		// print error staktrace
		errFmt := slogformatter.ErrorFormatter("error")
		fmts = append(fmts, errFmt)

		//Print time.Time in other format
		timeFmt := slogformatter.TimeFormatter(time.RFC3339, time.Now().Location())
		fmts = append(fmts, timeFmt)

		defaultHandler = slogformatter.NewFormatterHandler(fmts...)(consoleHan)
	} else {
		jsonHandler := slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{
			Level: level,
		})

		// formatters
		var fmts []slogformatter.Formatter
		timeFmt := slogformatter.TimeFormatter(time.RFC3339, time.UTC)
		fmts = append(fmts, timeFmt)
		defaultHandler = slogformatter.NewFormatterHandler(fmts...)(jsonHandler)
	}
	logger := slog.New(defaultHandler)
	return logger, nil
}

// SilentLogger returns a Zerologger that does not write any output
func SilentLogger() *slog.Logger {
	return slog.New(slog.NewTextHandler(io.Discard, &slog.HandlerOptions{}))
}

type SlogWriter struct {
	logger *slog.Logger
}

func (w SlogWriter) Write(p []byte) (n int, err error) {
	msg := strings.TrimRight(string(p), "\n") // Remove trailing newline
	w.logger.Debug(msg)
	return len(p), nil
}
