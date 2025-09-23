package main

import (
	cfg "github.com/conductorone/baton-keycloak/pkg/config"
	"github.com/conductorone/baton-sdk/pkg/config"
)

func main() {
	config.Generate("keycloak", cfg.Config)
}
