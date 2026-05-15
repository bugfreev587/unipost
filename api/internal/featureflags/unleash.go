package featureflags

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"strings"
	"time"

	unleash "github.com/Unleash/unleash-go-sdk/v6"
	unleashcontext "github.com/Unleash/unleash-go-sdk/v6/context"
)

type UnleashProvider struct {
	appName string
	env     string
}

func NewUnleashProviderFromEnv() (*UnleashProvider, error) {
	url := strings.TrimSpace(os.Getenv("UNLEASH_URL"))
	token := strings.TrimSpace(os.Getenv("UNLEASH_SERVER_TOKEN"))
	if url == "" {
		return nil, fmt.Errorf("UNLEASH_URL is required when FEATURE_FLAGS_PROVIDER=unleash")
	}
	if token == "" {
		return nil, fmt.Errorf("UNLEASH_SERVER_TOKEN is required when FEATURE_FLAGS_PROVIDER=unleash")
	}

	appName := strings.TrimSpace(os.Getenv("UNLEASH_APP_NAME"))
	if appName == "" {
		appName = "unipost-api"
	}
	env := strings.TrimSpace(os.Getenv("UNLEASH_ENVIRONMENT"))
	if env == "" {
		env = runtimeEnv()
	}

	if err := unleash.Initialize(
		unleash.WithAppName(appName),
		unleash.WithEnvironment(env),
		unleash.WithUrl(url),
		unleash.WithCustomHeaders(http.Header{"Authorization": {token}}),
		unleash.WithRefreshInterval(15*time.Second),
	); err != nil {
		return nil, err
	}

	return &UnleashProvider{appName: appName, env: env}, nil
}

func (p *UnleashProvider) Name() string {
	if p == nil {
		return "unleash"
	}
	return "unleash"
}

func (p *UnleashProvider) Close() error {
	unleash.Close()
	return nil
}

func (p *UnleashProvider) Enabled(_ context.Context, flag Flag, target Target, fallback bool) bool {
	ctx := unleashcontext.Context{
		UserId:        target.UserID,
		SessionId:     target.SessionID,
		RemoteAddress: target.RemoteAddress,
		Properties:    map[string]string{},
	}
	if target.UserEmail != "" {
		ctx.Properties["email"] = target.UserEmail
	}
	if target.WorkspaceID != "" {
		ctx.Properties["workspaceId"] = target.WorkspaceID
	}
	if target.Env != "" {
		ctx.Properties["env"] = target.Env
	}
	for k, v := range target.Properties {
		ctx.Properties[k] = v
	}

	return unleash.IsEnabled(string(flag), unleash.FeatureOptions{
		Ctx:      ctx,
		Fallback: &fallback,
	})
}

func runtimeEnv() string {
	env := strings.TrimSpace(os.Getenv("UNIPOST_ENV"))
	if env == "" {
		return "development"
	}
	return env
}
