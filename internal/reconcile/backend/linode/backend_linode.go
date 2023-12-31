package linode

import (
	"bytes"
	"context"
	"fmt"
	"net/http"
	"os"
	"strconv"
	"time"

	"github.com/cenkalti/backoff/v4"
	"github.com/linode/linodego"
	"github.com/pkg/errors"
	"go.uber.org/zap"
	"golang.org/x/oauth2"

	"slack-reconcile-deployments/internal/manifest"
	"slack-reconcile-deployments/internal/reconcile/backend"
	"slack-reconcile-deployments/internal/ssh"
)

// ErrNoInstanceFound indicates no instance found for when filtering by tags for ec2 instance.
var ErrNoInstanceFound = errors.New("no instance found")

// verify backend implements interface for backends
var _ backend.ProviderBackendReconciler = &ProviderBackend{}

// ProviderBackend is the ec2 specific provider backend
type ProviderBackend struct {
	log      *zap.SugaredLogger
	Client   *linodego.Client
	Manifest *manifest.Manifest
	// ssh client is lazily loaded once backend is running
	ssh           *ssh.Client
	PrivateKey    []byte
	HostID        string
	PublicDNSName string
	PublicKey     []byte
}

// New creates a new provider backend.
// Dependencies on ec2 and the desired manifest are expected.
func New(log *zap.SugaredLogger, ctx context.Context,
	manifest *manifest.Manifest) (backend.ProviderBackendReconciler, error) {
	privateKeyPath := manifest.Parameters["private-key-path"]
	if _, err := os.Stat(privateKeyPath); os.IsNotExist(err) {
		return nil, errors.Errorf("private key file %s does not exist", privateKeyPath)
	}

	privateKey, err := os.ReadFile(privateKeyPath)
	if err != nil {
		return nil, errors.Wrapf(err, "error reading private key file %s", privateKeyPath)
	}

	publicKeyPath := manifest.Parameters["public-key-path"]
	if _, err := os.Stat(publicKeyPath); os.IsNotExist(err) {
		return nil, errors.Errorf("private key file %s does not exist", publicKeyPath)
	}

	publicKey, err := os.ReadFile(publicKeyPath)
	if err != nil {
		return nil, errors.Wrapf(err, "error reading public key file %s", publicKeyPath)
	}

	log.Info("creating linode client")
	apiKey, ok := os.LookupEnv("LINODE_TOKEN")
	if !ok {
		return nil, errors.New("Could not find LINODE_TOKEN, please assert it is set.")
	}
	tokenSource := oauth2.StaticTokenSource(&oauth2.Token{AccessToken: apiKey})

	oauth2Client := &http.Client{
		Transport: &oauth2.Transport{
			Source: tokenSource,
		},
	}

	client := linodego.NewClient(oauth2Client)
	client.SetDebug(true)
	log.Infof("client: %+v", client)
	return &ProviderBackend{
		log:        log,
		Client:     &client,
		Manifest:   manifest,
		PrivateKey: privateKey,
		PublicKey:  publicKey,
	}, nil
}

// Run reconciles provider state with desired state
func (p *ProviderBackend) Run(ctx context.Context) (*ssh.Client, error) {
	// when exists, move on, we'll wait for running state later
	instanceID, publicDNSName, err := p.reconcile(ctx)
	if err != nil {
		return nil, errors.Wrap(err, "error reconciling ec2")
	}
	p.HostID = instanceID
	p.PublicDNSName = publicDNSName
	p.log.Infof("instance %s, %s, %s exists", instanceID, p.Manifest.ID, publicDNSName)

	p.ssh, err = ssh.New(p.log, true, p.PrivateKey,
		p.Username(), "", fmt.Sprintf("%s:22", publicDNSName))
	if err != nil {
		return nil, errors.Wrap(err, "error creating ssh client")
	}
	return p.ssh, nil
}

// WaitForRunning waits until an instance exists and is in running state
func (p *ProviderBackend) WaitForRunning(ctx context.Context, instanceID int) error {
	if err := backoff.Retry(func() error {
		out, err := p.Client.GetInstance(ctx,
			instanceID)
		if err != nil {
			return err
		}
		if out != nil {
			// really we expect one instance here
			instanceID := out.ID
			p.PublicDNSName = out.IPv4[0].String()
			p.log.Infof("checking instance state instanceId: %v, instanceState: %v", instanceID, out.Status)
			if out.Status != linodego.InstanceRunning {
				p.log.Infof("not running, instance found for %v", instanceID)
				return errors.Errorf("intance found, status: %v %v", instanceID, out.Status)
			}
			return nil
		}
		return ErrNoInstanceFound
	}, backoff.WithMaxRetries(backoff.NewConstantBackOff(10*time.Second), 15)); err != nil {
		return err
	}
	p.log.Infof("instance is running %s, %s, %s", instanceID, p.PublicDNSName, p.Manifest.ID)
	return nil
}

// Exists checks for an instance with given name
// TODO: add context timeout/stop
func (p *ProviderBackend) Exists(_ context.Context, name string) (bool, string, string, error) {
	exists := false
	instanceID := ""
	publicDNSName := ""

	if err := backoff.Retry(func() error {
		out, err := p.Client.ListInstances(context.Background(),
			linodego.NewListOptions(0, fmt.Sprintf(`{"id": %s}`, name)))
		fmt.Printf("%v", out)
		if err != nil {
			p.log.Warnf("%v", err)
			return err
		}

		// when reservations > 0, this is good, we found an instance
		if len(out) > 0 {
			for _, instance := range out {
				// skip terminated instances, they'll be cleaned up later
				if instance.Status == linodego.InstanceDeleting {
					p.log.Infof("instance %s is deleted/deleting", instance.ID)
					continue
				}
			}
			// instance exists, be done with the retries
			exists = true
			instanceID = strconv.Itoa(out[0].ID)
			publicDNSName = out[0].IPv4[0].String()
			return nil

		}
		return ErrNoInstanceFound
	}, backoff.WithMaxRetries(backoff.NewConstantBackOff(5*time.Second), 2)); err != nil {
		return false, "", "", err
	}
	return exists, instanceID, publicDNSName, nil
}

// Close closes the ssh client and session
func (p *ProviderBackend) Close() {
	if p.ssh != nil {
		p.ssh.Close()
	}
}

// reconcile creates an instance if it does not exist.  instance
func (p *ProviderBackend) reconcile(ctx context.Context) (string, string, error) {
	exists, instanceID, publicDNSName, err := p.Exists(ctx, p.Manifest.ID)
	if err != nil {
		if !errors.Is(err, ErrNoInstanceFound) {
			return "", "", errors.Wrapf(err, "error checking for instance %s", p.Manifest.ID)
		}
	}

	if exists {
		p.log.Infof("instance %s exists, %s", instanceID, publicDNSName)
		return instanceID, publicDNSName, nil
	}

	//publicKey, err := ssh.PublicKeyFromPrivateKey(p.PrivateKey)
	//if err != nil {
	//	return "", "", errors.Wrap(err, "error getting public key from private key")
	//}
	//fmt.Printf("public key: %s", string(publicKey.Marshal()))
	out, err := p.Client.CreateInstance(ctx, linodego.InstanceCreateOptions{
		Region:          p.Manifest.Parameters["region"],
		Type:            p.Manifest.Parameters["size"],
		Label:           p.Manifest.ID,
		Group:           "",
		AuthorizedKeys:  []string{string(bytes.TrimSpace(p.PublicKey))},
		AuthorizedUsers: nil,
		Image:           p.Manifest.Parameters["image-id"],
		Interfaces:      nil,
		BackupsEnabled:  false,
		PrivateIP:       false,
		Tags:            nil,
	})
	if err != nil {
		return "", "", errors.Wrapf(err, "error creating instance %s", p.Manifest.ID)
	}

	p.log.Infof("created instance %+v", out)
	instanceID = strconv.Itoa(out.ID)
	p.HostID = instanceID

	p.log.Infof("waiting for instance %s to be running %v", p.HostID, p.Manifest.ID)
	if err := p.WaitForRunning(ctx, out.ID); err != nil {
		return "", "", errors.Wrapf(err, "error waiting for instance %s", p.Manifest.ID)
	}
	p.log.Infof("instance %s is running", instanceID)
	return instanceID, p.PublicDNSName, nil
}

// Username provide the username for this provider. Each provider can specify a
// unique username. The ec2 username will be admin.
func (p *ProviderBackend) Username() string {
	return "root"
}

// Password not used by this backend
func (p *ProviderBackend) Password() string {
	return ""
}

// WithOption not used by this backend
func (p *ProviderBackend) WithOption(_ string, _ string) {
	// not implemented on this backend, does not use password auth
}
