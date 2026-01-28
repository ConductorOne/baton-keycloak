package main

import (
	"context"

	cfg "github.com/conductorone/baton-keycloak/pkg/config"

	"github.com/conductorone/baton-keycloak/pkg/connector"
	"github.com/conductorone/baton-sdk/pkg/config"
)

var version = "dev"

func main() {
	ctx := context.Background()
	config.RunConnector(ctx, "baton-keycloak", version, cfg.Config, connector.New)
}
