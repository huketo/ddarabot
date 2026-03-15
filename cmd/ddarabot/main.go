package main

import (
	"context"
	"flag"
	"fmt"
	"log/slog"
	"os"
	"os/signal"
	"strings"
	"syscall"

	coreapi "github.com/firebase/genkit/go/core/api"
	"github.com/firebase/genkit/go/genkit"
	"github.com/firebase/genkit/go/plugins/compat_oai/anthropic"
	"github.com/firebase/genkit/go/plugins/compat_oai/openai"
	"github.com/firebase/genkit/go/plugins/googlegenai"
	"github.com/firebase/genkit/go/plugins/ollama"
	"github.com/openai/openai-go/option"

	"github.com/huketo/ddarabot/internal/bluesky"
	"github.com/huketo/ddarabot/internal/bot"
	"github.com/huketo/ddarabot/internal/config"
	"github.com/huketo/ddarabot/internal/store"
	"github.com/huketo/ddarabot/internal/translator"
)

var version = "dev"

func main() {
	if len(os.Args) > 1 {
		switch os.Args[1] {
		case "version":
			fmt.Printf("ddarabot %s\n", version)
			return
		case "validate":
			os.Args = append(os.Args[:1], os.Args[2:]...)
			runValidate()
			return
		}
	}

	runBot()
}

func resolveConfigPath(fs *flag.FlagSet) string {
	cfgPath := "config.toml"
	fs.Visit(func(f *flag.Flag) {
		if f.Name == "c" || f.Name == "config" {
			cfgPath = f.Value.String()
		}
	})
	return cfgPath
}

func runBot() {
	fs := flag.NewFlagSet("ddarabot", flag.ExitOnError)
	fs.String("config", "config.toml", "path to config file")
	fs.String("c", "config.toml", "path to config file (short)")
	dryRun := fs.Bool("dry-run", false, "translate but do not post replies")
	fs.Parse(os.Args[1:])

	cfgPath := resolveConfigPath(fs)

	cfg, err := config.Load(cfgPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}

	logger := setupLogger(cfg.Log.Level)

	ctx, cancel := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer cancel()

	g := initGenkit(ctx, &cfg.LLM, logger)

	st, err := store.New(cfg.Store.Path)
	if err != nil {
		logger.Error("store init failed", "error", err)
		os.Exit(1)
	}
	defer st.Close()

	did, err := bluesky.ResolveDID(ctx, cfg.Bluesky.PDSHost, cfg.Bluesky.Handle)
	if err != nil {
		logger.Error("failed to resolve DID", "handle", cfg.Bluesky.Handle, "error", err)
		os.Exit(1)
	}
	logger.Info("resolved DID", "handle", cfg.Bluesky.Handle, "did", did)

	auth := bluesky.NewAuth(cfg.Bluesky.PDSHost, cfg.Bluesky.Handle, cfg.Bluesky.AppPassword)
	poster := bluesky.NewPoster(auth, cfg.Bluesky.PDSHost, logger, *dryRun)
	tr := translator.New(g, cfg.LLM.Model, cfg.Translation.Footer, cfg.Translation.SummarizeOnOverflow, logger)

	b := bot.New(cfg, did, st, tr, poster, logger)

	logger.Info("starting ddarabot",
		"version", version,
		"model", cfg.LLM.Model,
		"targets", cfg.Translation.TargetLanguages,
		"dry-run", *dryRun,
	)

	if err := b.Run(ctx); err != nil && err != context.Canceled {
		logger.Error("bot stopped", "error", err)
		os.Exit(1)
	}

	logger.Info("ddarabot stopped gracefully")
}

func runValidate() {
	fs := flag.NewFlagSet("validate", flag.ExitOnError)
	fs.String("config", "config.toml", "path to config file")
	fs.String("c", "config.toml", "path to config file (short)")
	fs.Parse(os.Args[1:])

	cfgPath := resolveConfigPath(fs)

	cfg, err := config.Load(cfgPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "config error: %v\n", err)
		os.Exit(1)
	}
	fmt.Println("config OK")

	ctx := context.Background()
	logger := slog.Default()
	g := initGenkit(ctx, &cfg.LLM, logger)

	tr := translator.New(g, cfg.LLM.Model, cfg.Translation.Footer, cfg.Translation.SummarizeOnOverflow, logger)
	results, errs := tr.TranslateAll(ctx, "Hello", cfg.Translation.SourceLanguage, cfg.Translation.TargetLanguages[:1])
	if len(errs) > 0 {
		fmt.Fprintf(os.Stderr, "LLM test error: %v\n", errs[0])
		os.Exit(1)
	}
	for lang, text := range results {
		fmt.Printf("LLM test OK (%s): %s\n", lang, text)
	}
}

func initGenkit(ctx context.Context, cfg *config.LLM, logger *slog.Logger) *genkit.Genkit {
	provider := cfg.Provider()

	var plugin coreapi.Plugin
	switch provider {
	case "openai":
		plugin = &openai.OpenAI{APIKey: cfg.OpenAI.APIKey}
	case "anthropic":
		plugin = &anthropic.Anthropic{
			Opts: []option.RequestOption{option.WithAPIKey(cfg.Anthropic.APIKey)},
		}
	case "googleai":
		plugin = &googlegenai.GoogleAI{APIKey: cfg.GoogleAI.APIKey}
	case "vertexai":
		plugin = &googlegenai.VertexAI{ProjectID: cfg.VertexAI.ProjectID, Location: cfg.VertexAI.Location}
	case "ollama":
		plugin = &ollama.Ollama{ServerAddress: cfg.Ollama.ServerAddress, Timeout: cfg.Ollama.Timeout}
	default:
		logger.Error("unsupported LLM provider", "provider", provider, "model", cfg.Model)
		os.Exit(1)
	}

	return genkit.Init(ctx, genkit.WithPlugins(plugin))
}

func setupLogger(level string) *slog.Logger {
	var logLevel slog.Level
	switch strings.ToLower(level) {
	case "debug":
		logLevel = slog.LevelDebug
	case "warn":
		logLevel = slog.LevelWarn
	case "error":
		logLevel = slog.LevelError
	default:
		logLevel = slog.LevelInfo
	}
	return slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: logLevel}))
}
