package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"strings"

	"github.com/jetstack/cert-manager/pkg/acme/webhook/apis/acme/v1alpha1"
	"github.com/jetstack/cert-manager/pkg/acme/webhook/cmd"
	certmanagerv1 "github.com/jetstack/cert-manager/pkg/apis/meta/v1"
	"github.com/jetstack/cert-manager/pkg/issuer/acme/dns/util"
	"github.com/ndemeshchenko/dnsmadeasy"
	extAPI "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	metaV1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
)

const (
	providerName    = "dnsmadeasy"
	defaultTTL      = 600
	groupNameEnvVar = "GROUP_NAME"
)

func main() {
	groupName := os.Getenv("GROUP_NAME")
	if groupName == "" {
		panic(fmt.Sprintf("%s must be specified", groupNameEnvVar))
	}

	cmd.RunWebhookServer(groupName,
		&dnsmadeasyDNSProviderSolver{},
	)
}

type dnsmadeasyDNSProviderSolver struct {
	client *kubernetes.Clientset
}

type dnsmadeasyDNSProviderConfig struct {
	APIKeyRef    certmanagerv1.SecretKeySelector `json:"apiKeyRef"`
	APISecretRef certmanagerv1.SecretKeySelector `json:"apiSecretRef"`
	TTL          *int                            `json:"ttl"`
	Sandbox      bool                            `json:"sandbox"`
}

func (c *dnsmadeasyDNSProviderSolver) validate(cfg *dnsmadeasyDNSProviderConfig) error {
	if cfg.APIKeyRef.Name == "" {
		return errors.New("API Key field is not provided")
	}

	if cfg.APISecretRef.Name == "" {
		return errors.New("API Secret field is not provided")
	}
	return nil
}

// Name function returns DNS provider name.
func (c *dnsmadeasyDNSProviderSolver) Name() string {
	return providerName
}

func (c *dnsmadeasyDNSProviderSolver) Present(ch *v1alpha1.ChallengeRequest) error {
	fmt.Printf("\n>>>Present: fqdn:[%s] zone:[%s]\n", ch.ResolvedFQDN, ch.ResolvedZone)

	cfg, err := loadConfig(ch.Config)
	if err != nil {
		printError(err)
		return err
	}

	provider, err := c.provider(&cfg, ch.ResourceNamespace)
	if err != nil {
		printError(err)
		return err
	}

	domainID, err := getDomainID(provider, ch.ResolvedZone)
	if err != nil {
		printError(err)
		return err
	}

	existingRecord, err := findTXTRecord(provider, domainID, ch.ResolvedZone, ch.ResolvedFQDN, ch.Key)
	if err != nil {
		printError(err)
		return err
	}

	if existingRecord == nil {
		name := fetchRecordName(ch.ResolvedFQDN, ch.ResolvedZone)
		r := &dnsmadeasy.Record{
			Name:        name,
			Type:        "TXT",
			Value:       ch.Key,
			GtdLocation: "DEFAULT",
			TTL:         *cfg.TTL,
		}
		_, err := provider.CreateRecord(domainID, r)
		if err != nil {
			printError(err)
			return fmt.Errorf("DnsMadeEasy API call failed: %v", err)
		}
	} else {
		existingRecord.TTL = *cfg.TTL

		err = provider.UpdateRecord(domainID, existingRecord)
		if err != nil {
			printError(err)
			return fmt.Errorf("DNSMadeEasy API call failed: %v", err)
		}
	}

	fmt.Printf("\n<<<Present: fqdn:[%s] zone:[%s]\n", ch.ResolvedFQDN, ch.ResolvedZone)
	return nil
}

func (c *dnsmadeasyDNSProviderSolver) CleanUp(ch *v1alpha1.ChallengeRequest) error {
	// TODO
	fmt.Printf("\n>>>CleanUp(): fqdn:[%s] zone:[%s]\n", ch.ResolvedFQDN, ch.ResolvedZone)
	cfg, err := loadConfig(ch.Config)
	if err != nil {
		printError(err)
		return err
	}

	provider, err := c.provider(&cfg, ch.ResourceNamespace)
	if err != nil {
		printError(err)
		return err
	}

	domainID, err := getDomainID(provider, ch.ResolvedZone)
	if err != nil {
		printError(err)
		return err
	}

	existingRecord, err := findTXTRecord(provider, domainID, ch.ResolvedZone, ch.ResolvedFQDN, ch.Key)
	if err != nil {
		printError(err)
		return err
	}

	if existingRecord != nil {
		err = provider.DeleteRecord(domainID, existingRecord.ID)
		if err != nil {
			printError(err)
			return fmt.Errorf("DnsMadeEasy API call failed: %v", err)
		}
	}

	return nil

}

// Initialize will be called when the webhook first starts.
// This method can be used to instantiate the webhook, i.e. initialising
// connections or warming up caches.
// Typically, the kubeClientConfig parameter is used to build a Kubernetes
// client that can be used to fetch resources from the Kubernetes API, e.g.
// Secret resources containing credentials used to authenticate with DNS
// provider accounts.
// The stopCh can be used to handle early termination of the webhook, in cases
// where a SIGTERM or similar signal is sent to the webhook process.
func (c *dnsmadeasyDNSProviderSolver) Initialize(kubeClientCfg *rest.Config, stopCh <-chan struct{}) error {
	client, err := kubernetes.NewForConfig(kubeClientCfg)
	if err != nil {
		return err
	}

	c.client = client
	return nil
}

func (c *dnsmadeasyDNSProviderSolver) provider(cfg *dnsmadeasyDNSProviderConfig, namespace string) (*dnsmadeasy.DMEClient, error) {
	err := c.validate(cfg)
	if err != nil {
		return nil, err
	}

	keysec := make(map[string]string)
	for key, ref := range map[string]certmanagerv1.SecretKeySelector{"APIKeyRef": cfg.APIKeyRef, "APISecretRef": cfg.APISecretRef} {
		secret, err := c.client.CoreV1().
			Secrets(namespace).
			Get(context.Background(), ref.Name, metaV1.GetOptions{})

		if err != nil {
			return nil, err
		}

		secretBytes, ok := secret.Data[ref.Key]
		if !ok {
			return nil, fmt.Errorf("no %s for %q in secret '%s/%s'", key, ref.Name, namespace, ref.Key)
		}

		keysec[key] = string(secretBytes)

	}

	providerConfig, err := dnsmadeasy.New(&dnsmadeasy.DMEClient{
		APIAccessKey: keysec["APIKeyRef"],
		APISecretKey: keysec["APISecretRef"],
	})

	if err != nil {
		return nil, err
	}

	return providerConfig, err
}

func getDomainID(client *dnsmadeasy.DMEClient, zone string) (int, error) {
	domains, err := client.Domains()
	if err != nil {
		return -1, fmt.Errorf("DnsMadeEasy API call failed: %v", err)
	}

	authZone, err := util.FindZoneByFqdn(zone, util.RecursiveNameservers)
	if err != nil {
		return -1, err
	}

	var hostedDomain dnsmadeasy.Domain
	for _, domain := range domains {
		if domain.Name == util.UnFqdn(authZone) {
			hostedDomain = domain
			break
		}
	}

	if hostedDomain.ID == 0 {
		return -1, fmt.Errorf("zone %s not found in DnsMadeEasy for zone %s", authZone, zone)
	}

	return hostedDomain.ID, err
}

func findTXTRecord(client *dnsmadeasy.DMEClient, domainID int, zone, fqdn, key string) (*dnsmadeasy.Record, error) {
	name := fetchRecordName(fqdn, zone)
	records, err := client.Records(domainID)
	if err != nil {
		return nil, fmt.Errorf("DnsMadeEasy API call failed: %v", err)
	}

	for _, record := range records {
		if record.Name == name && record.Type == "TXT" && trimQuotes(record.Value) == key {
			fmt.Printf("DNS record found %v\n", record)
			return &record, nil
		}
	}

	return nil, nil
}

func fetchRecordName(fqdn, zone string) string {
	if idx := strings.Index(fqdn, "."+zone); idx != -1 {
		return fqdn[:idx]
	}

	return util.UnFqdn(fqdn)
}

// DnsMadeEasy wraps TXT value with some unnecessary quotes which break the match
// trimQuotes method removes them
func trimQuotes(s string) string {
	if len(s) >= 2 {
		if s[0] == '"' && s[len(s)-1] == '"' {
			return s[1 : len(s)-1]
		}
	}
	return s
}

// loadConfig is a small helper function that decodes JSON configuration into
// the typed config struct.
func loadConfig(cfgJSON *extAPI.JSON) (dnsmadeasyDNSProviderConfig, error) {
	ttl := defaultTTL
	cfg := dnsmadeasyDNSProviderConfig{TTL: &ttl}

	if cfgJSON == nil {
		return cfg, nil
	}

	err := json.Unmarshal(cfgJSON.Raw, &cfg)
	if err != nil {
		return cfg, fmt.Errorf("can't decode DNS01 solver config: %v", err)
	}

	return cfg, nil
}

func printError(err error) {
	fmt.Printf("\n\nERROR\n %v \n\n", err)
}
