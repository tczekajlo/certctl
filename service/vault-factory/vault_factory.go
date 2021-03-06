package vaultfactory

import (
	"net/http"

	vaultclient "github.com/hashicorp/vault/api"

	"github.com/giantswarm/certctl/service/spec"
)

// Config represents the configuration used to create a new Vault factory.
type Config struct {
	// Dependencies.
	HTTPClient *http.Client

	// Settings.
	Address    string
	AdminToken string
}

// DefaultConfig provides a default configuration to create a Vault factory.
func DefaultConfig() Config {
	newConfig := Config{
		// Dependencies.
		HTTPClient: http.DefaultClient,

		// Settings.
		Address:    "http://127.0.0.1:8200",
		AdminToken: "admin-token",
	}

	return newConfig
}

// New creates a new configured Vault factory.
func New(config Config) (spec.VaultFactory, error) {
	newVaultFactory := &vaultFactory{
		Config: config,
	}

	// Dependencies.
	if newVaultFactory.Address == "" {
		return nil, maskAnyf(invalidConfigError, "Vault address must not be empty")
	}
	// Settings.
	if newVaultFactory.HTTPClient == nil {
		return nil, maskAnyf(invalidConfigError, "HTTP client must not be empty")
	}
	if newVaultFactory.AdminToken == "" {
		return nil, maskAnyf(invalidConfigError, "Vault admin token must not be empty")
	}

	return newVaultFactory, nil
}

type vaultFactory struct {
	Config
}

func (vf *vaultFactory) NewClient() (*vaultclient.Client, error) {
	newClientConfig := vaultclient.DefaultConfig()
	newClientConfig.Address = vf.Address
	newClientConfig.HttpClient = vf.HTTPClient
	newVaultClient, err := vaultclient.NewClient(newClientConfig)
	if err != nil {
		return nil, maskAny(err)
	}
	newVaultClient.SetToken(vf.AdminToken)

	return newVaultClient, nil
}
