package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/xiaoboyu/unipost-api/internal/platformcontrols"
)

type commandOptions struct {
	platform        string
	state           platformcontrols.State
	expectedVersion int64
	reason          string
}

func parseOptions(args []string) (commandOptions, error) {
	flags := flag.NewFlagSet("platform-control", flag.ContinueOnError)
	flags.SetOutput(io.Discard)
	var options commandOptions
	var state string
	flags.StringVar(&options.platform, "platform", "", "platform to update")
	flags.StringVar(&state, "state", "", "active, publishing_disabled, or fully_disabled")
	flags.Int64Var(&options.expectedVersion, "expected-version", 0, "current control version")
	flags.StringVar(&options.reason, "reason", "", "auditable reason code")
	if err := flags.Parse(args); err != nil {
		return commandOptions{}, err
	}
	platformName, err := platformcontrols.NormalizePlatform(options.platform)
	if err != nil {
		if strings.TrimSpace(options.platform) == "" {
			return commandOptions{}, errors.New("--platform is required")
		}
		return commandOptions{}, err
	}
	options.platform = platformName
	options.state = platformcontrols.State(strings.ToLower(strings.TrimSpace(state)))
	if !options.state.Valid() {
		return commandOptions{}, errors.New("--state must be active, publishing_disabled, or fully_disabled")
	}
	if options.expectedVersion < 1 {
		return commandOptions{}, errors.New("--expected-version must be positive")
	}
	options.reason = strings.TrimSpace(options.reason)
	if options.reason == "" {
		return commandOptions{}, errors.New("--reason is required")
	}
	return options, nil
}

func run(ctx context.Context, args []string, databaseURL string, output io.Writer) error {
	options, err := parseOptions(args)
	if err != nil {
		return err
	}
	if strings.TrimSpace(databaseURL) == "" {
		return errors.New("DATABASE_URL is required")
	}
	pool, err := pgxpool.New(ctx, databaseURL)
	if err != nil {
		return errors.New("failed to configure database connection")
	}
	defer pool.Close()
	if err := pool.Ping(ctx); err != nil {
		return errors.New("failed to connect to database")
	}
	service := platformcontrols.NewService(platformcontrols.NewPostgresStore(pool))
	result, err := service.SetState(ctx, platformcontrols.TransitionRequest{
		Platform:        options.platform,
		State:           options.state,
		ExpectedVersion: options.expectedVersion,
		ReasonCode:      options.reason,
		RequestID:       "platform-control-cli",
		ActorUserAgent:  "platform-control-cli",
	})
	var conflict *platformcontrols.VersionConflictError
	if errors.As(err, &conflict) {
		return fmt.Errorf("version conflict: current state=%s version=%d", conflict.Current.State, conflict.Current.Version)
	}
	if err != nil {
		return errors.New("failed to update platform operational control")
	}
	_, err = fmt.Fprintf(output, "platform=%s state=%s version=%d changed=%t\n",
		result.Control.Platform, result.Control.State, result.Control.Version, result.Changed)
	return err
}

func main() {
	if err := run(context.Background(), os.Args[1:], os.Getenv("DATABASE_URL"), os.Stdout); err != nil {
		fmt.Fprintln(os.Stderr, "platform-control:", err)
		os.Exit(1)
	}
}
