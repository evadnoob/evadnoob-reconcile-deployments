package ec2

import (
	"context"
	"fmt"

	"github.com/pkg/errors"
	"go.uber.org/zap"

	"slack-reconcile-deployments/internal/manifest"
	"slack-reconcile-deployments/internal/reconcile/backend"
	"slack-reconcile-deployments/internal/ssh"
)

// verify backend implements interface for backends
var _ backend.ProviderBackendReconciler = &ProviderBackend{}

// ProviderBackend is the slack specific backend
type ProviderBackend struct {
	log *zap.SugaredLogger
	// Manifest is the manifest for the deployment
	Manifest *manifest.Manifest
	// ssh client is lazily loaded once backend is running
	ssh *ssh.Client
	// HostID is the host id for the instance
	HostID string
	// PublicDNSName is the public dns name for the instance
	PublicDNSName string
	// password used for ssh auth
	password string
}

// New creates a new backend.
// Dependencies on ec2 and the desired manifest are expected.
func New(log *zap.SugaredLogger, manifest *manifest.Manifest) backend.ProviderBackendReconciler {
	return &ProviderBackend{
		log:      log,
		Manifest: manifest,
	}
}

// Run reconciles backend state with desired state
func (p *ProviderBackend) Run(_ context.Context) (*ssh.Client, error) {
	host := p.Manifest.Parameters["hostname"]
	var err error
	p.ssh, err = ssh.New(p.log, true, []byte{},
		p.Username(), p.password, fmt.Sprintf("%s:22", host))
	if err != nil {
		return nil, errors.Wrap(err, "error creating ssh client")
	}
	p.log.Infof("slack backend for %s done", host)
	return p.ssh, nil
}

func (p *ProviderBackend) Close() {
	if p.ssh != nil {
		p.ssh.Close()
	}
}

// Username provide the username for this provider. Each provider can specify a
// unique username. The slack username will be root
func (p *ProviderBackend) Username() string {
	return "root"
}

// Password is the password for ssh auth
func (p *ProviderBackend) Password() string {
	return p.password
}

func (p *ProviderBackend) WithOption(name string, value string) {
	switch name { // nolint:gocritic //singleCaseSwitch  planning to add more cases
	case "password":
		p.password = value
	}
}
