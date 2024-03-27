package main

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"syscall"

	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"

	"github.com/criteo/data-aggregation-api/internal/api/auth"
	"github.com/criteo/data-aggregation-api/internal/api/router"
	"github.com/criteo/data-aggregation-api/internal/app"
	"github.com/criteo/data-aggregation-api/internal/config"
	"github.com/criteo/data-aggregation-api/internal/convertor/device"
	"github.com/criteo/data-aggregation-api/internal/job"
	"github.com/criteo/data-aggregation-api/internal/report"
)

func configureLogging(logLevel string, pretty bool) error {
	level, err := zerolog.ParseLevel(logLevel)
	if err != nil {
		return fmt.Errorf("failed to parse log level '%s': %w", logLevel, err)
	}
	zerolog.SetGlobalLevel(level)
	zerolog.TimeFieldFormat = zerolog.TimeFormatUnix //nolint:reassign // it is the way
	if pretty {
		log.Logger = log.Output(zerolog.ConsoleWriter{Out: os.Stderr}) //nolint:reassign // it is the way
	}

	return nil
}

var (
	version = ""
	date    = "unknown"
	commit  = "unknown"
	builtBy = "unknown"
)

func dispatchSingleRequest(incoming chan struct{}) chan bool {
	outgoing := make(chan bool)

	go func() {
		defer close(outgoing)
		for range incoming {
			log.Info().Msg("Received new build request.")
			outgoing <- true
		}
	}()

	return outgoing
}

func run() error {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	if err := config.LoadConfig(); err != nil {
		return err
	}
	if err := configureLogging(config.Cfg.Log.Level, config.Cfg.Log.Pretty); err != nil {
		return err
	}

	log.Info().Str("version", version).Send()
	log.Info().Str("commit", commit).Send()
	log.Info().Str("build-time", date).Send()
	log.Info().Str("build-user", builtBy).Send()

	// Configure LDAP timeout
	if config.Cfg.Authentication.LDAP != nil {
		if config.Cfg.Authentication.LDAP.Timeout <= 0 {
			return fmt.Errorf("LDAP timeout must be greater than 0: %d", config.Cfg.Authentication.LDAP.Timeout)
		}
		auth.SetLDAPDefaultTimeout(config.Cfg.Authentication.LDAP.Timeout)
	}

	deviceRepo := device.NewSafeRepository()
	reports := report.NewRepository()

	newBuildRequest := make(chan struct{})
	triggerNewBuild := dispatchSingleRequest(newBuildRequest)

	go job.StartBuildLoop(&deviceRepo, &reports, triggerNewBuild)
	if err := router.NewManager(&deviceRepo, &reports, newBuildRequest).ListenAndServe(ctx, config.Cfg.API.ListenAddress, config.Cfg.API.ListenPort); err != nil {
		return fmt.Errorf("webserver error: %w", err)
	}

	return nil
}

func main() {
	app.Info.Version = version
	app.Info.BuildTime = date
	app.Info.BuildUser = builtBy
	app.Info.Commit = commit

	if err := run(); err != nil {
		log.Fatal().Err(err).Send()
	}
}
