package cli

import (
	"fmt"
	"log"
	"net/http"

	"github.com/spf13/cobra"

	"github.com/giantswarm/certctl/service/pki"
	"github.com/giantswarm/certctl/service/token"
	"github.com/giantswarm/certctl/service/vault-factory"
)

type setupFlags struct {
	// Vault
	VaultAddress string
	VaultToken   string

	// Cluster
	ClusterID string

	// PKI
	AllowedDomains   string
	CommonName       string
	CATTL            string
	AllowBareDomains bool

	// Token
	NumTokens int
	TokenTTL  string
}

var (
	setupCmd = &cobra.Command{
		Use:   "setup",
		Short: "Setup a Vault PKI backend including all necessary requirements.",
		Run:   setupRun,
	}

	newSetupFlags = &setupFlags{}
)

func init() {
	CLICmd.AddCommand(setupCmd)

	setupCmd.Flags().StringVar(&newSetupFlags.VaultAddress, "vault-addr", fromEnv("VAULT_ADDR", "http://127.0.0.1:8200"), "Address used to connect to Vault.")
	setupCmd.Flags().StringVar(&newSetupFlags.VaultToken, "vault-token", fromEnv("VAULT_TOKEN", ""), "Token used to authenticate against Vault.")

	setupCmd.Flags().StringVar(&newSetupFlags.ClusterID, "cluster-id", "", "Cluster ID used to generate a new root CA for.")

	setupCmd.Flags().StringVar(&newSetupFlags.AllowedDomains, "allowed-domains", "", "Comma separated domains allowed to authenticate against the cluster's root CA.")
	setupCmd.Flags().StringVar(&newSetupFlags.CommonName, "common-name", "", "Common name used to generate a new root CA for.")
	setupCmd.Flags().StringVar(&newSetupFlags.CATTL, "ca-ttl", "86400h", "TTL used to generate a new root CA.") // 10 years
	setupCmd.Flags().BoolVar(&newSetupFlags.AllowBareDomains, "allow-bare-domains", false, "Allow issuing certs for bare domains. (Default false)")

	setupCmd.Flags().IntVar(&newSetupFlags.NumTokens, "num-tokens", 1, "Number of tokens to generate.")
	setupCmd.Flags().StringVar(&newSetupFlags.TokenTTL, "token-ttl", "720h", "TTL used to generate new tokens.")
}

func setupValidate(newSetupFlags *setupFlags) error {
	if newSetupFlags.VaultToken == "" {
		return maskAnyf(invalidConfigError, "Vault token must not be empty")
	}
	if newSetupFlags.AllowedDomains == "" {
		return maskAnyf(invalidConfigError, "allowed domains must not be empty")
	}
	if newSetupFlags.ClusterID == "" {
		return maskAnyf(invalidConfigError, "cluster ID must not be empty")
	}
	if newSetupFlags.CommonName == "" {
		return maskAnyf(invalidConfigError, "common name must not be empty")
	}

	return nil
}

func setupRun(cmd *cobra.Command, args []string) {
	err := setupValidate(newSetupFlags)
	if err != nil {
		log.Fatalf("%#v\n", maskAny(err))
	}

	// Create a Vault client factory.
	newVaultFactoryConfig := vaultfactory.DefaultConfig()
	newVaultFactoryConfig.HTTPClient = &http.Client{}
	newVaultFactoryConfig.Address = newSetupFlags.VaultAddress
	newVaultFactoryConfig.AdminToken = newSetupFlags.VaultToken
	newVaultFactory, err := vaultfactory.New(newVaultFactoryConfig)
	if err != nil {
		log.Fatalf("%#v\n", maskAny(err))
	}

	// Create a Vault client and configure it with the provided admin token
	// through the factory.
	newVaultClient, err := newVaultFactory.NewClient()
	if err != nil {
		log.Fatalf("%#v\n", maskAny(err))
	}

	// Create a PKI controller to setup the cluster's PKI backend including its
	// root CA and role.
	var pkiService pki.Service
	{
		pkiConfig := pki.DefaultServiceConfig()
		pkiConfig.VaultClient = newVaultClient
		pkiService, err = pki.NewService(pkiConfig)
		if err != nil {
			log.Fatalf("%#v\n", maskAny(err))
		}
	}

	// Create a token generator to create new tokens for the current cluster.
	var tokenService token.Service
	{
		tokenConfig := token.DefaultServiceConfig()
		tokenConfig.VaultClient = newVaultClient
		tokenService, err = token.NewService(tokenConfig)
		if err != nil {
			log.Fatalf("%#v\n", maskAny(err))
		}
	}

	// Setup PKI backend for cluster.
	{
		createConfig := pki.CreateConfig{
			AllowedDomains:   newSetupFlags.AllowedDomains,
			ClusterID:        newSetupFlags.ClusterID,
			CommonName:       newSetupFlags.CommonName,
			TTL:              newSetupFlags.CATTL,
			AllowBareDomains: newSetupFlags.AllowBareDomains,
		}
		err = pkiService.Create(createConfig)
		if err != nil {
			log.Fatalf("%#v\n", maskAny(err))
		}
	}

	// Generate tokens for the cluster VMs.
	var tokens []string
	{
		createConfig := token.CreateConfig{
			ClusterID: newSetupFlags.ClusterID,
			Num:       newSetupFlags.NumTokens,
			TTL:       newSetupFlags.TokenTTL,
		}
		tokens, err = tokenService.Create(createConfig)
		if err != nil {
			log.Fatalf("%#v\n", maskAny(err))
		}
	}

	fmt.Printf("Set up cluster for ID '%s':\n", newSetupFlags.ClusterID)
	fmt.Printf("\n")
	fmt.Printf("    - PKI backend mounted\n")
	fmt.Printf("    - Root CA generated\n")
	fmt.Printf("    - PKI role created\n")
	fmt.Printf("    - PKI policy created\n")
	fmt.Printf("\n")
	fmt.Printf("The following tokens have been generated for this cluster:\n")
	fmt.Printf("\n")
	for _, t := range tokens {
		fmt.Printf("    %s\n", t)
	}
	fmt.Printf("\n")
}
