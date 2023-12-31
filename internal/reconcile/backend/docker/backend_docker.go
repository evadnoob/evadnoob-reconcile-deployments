package docker

import (
	"context"
	"fmt"
	"os"
	"strconv"
	"strings"

	"github.com/davecgh/go-spew/spew"
	"github.com/ory/dockertest/v3"
	dc "github.com/ory/dockertest/v3/docker"
	"github.com/pkg/errors"
	"go.uber.org/zap"

	"slack-reconcile-deployments/internal/freeport"
	"slack-reconcile-deployments/internal/manifest"
	"slack-reconcile-deployments/internal/pkc"
	"slack-reconcile-deployments/internal/reconcile/backend"
	"slack-reconcile-deployments/internal/ssh"
)

// ProviderBackend is the docker specific provider backend
type ProviderBackend struct {
	log *zap.SugaredLogger
	// container is the docker container used for testing
	container *dockertest.Resource
	// manifest is the manifest for the deployment
	manifest *manifest.Manifest
	// ssh is the ssh client connected the container
	ssh *ssh.Client
	// privateKey is the private key used for ssh auth
	privateKey []byte
}

// verify backend implements interface for backends
var _ backend.ProviderBackendReconciler = &ProviderBackend{}

// New creates a docker provider backend
func New(log *zap.SugaredLogger, m *manifest.Manifest) backend.ProviderBackendReconciler {
	return &ProviderBackend{
		log:      log,
		manifest: m,
	}
}

// Close closes the docker container
func (p *ProviderBackend) Close() {
	if p.container == nil {
		return
	}

	// expire is required to ensure container is removed(purge is not enough)
	if err := p.container.Expire(1); err != nil {
		p.log.Errorf("error expiring container %v %v", p.container, err)
	}

	if err := p.container.Close(); err != nil {
		p.log.Errorf("error stopping container %v %v", p.container, err)
	}
}

// Run runs localstack in background for provider
func (p *ProviderBackend) Run(_ context.Context) (*ssh.Client, error) {
	p.log.Infof("run")

	pool, err := dockertest.NewPool("")
	if err != nil {
		return nil, errors.Wrap(err, "error on new dockertest pool")
	}

	err = pool.Client.Ping()
	if err != nil {
		return nil, errors.Wrap(err, "error ping docker")
	}

	sshPort, err := freeport.GetFreePort()
	if err != nil {
		return nil, errors.Wrap(err, "error getting free port")
	}

	cmd := []string{"/bin/bash", "-c", `apt-get update && apt-get install -y ca-certificates \
		 openssh-client openssh-server && mkdir /run/sshd && /usr/sbin/sshd -D -e -o \
         IgnoreUserKnownHosts=yes -o PermitEmptyPasswords=yes -o PermitRootLogin=yes`}

	container, err := pool.RunWithOptions(&dockertest.RunOptions{
		Repository:   "debian",
		Tag:          "12.2",
		Cmd:          cmd,
		ExposedPorts: []string{"22/tcp"},
		PortBindings: map[dc.Port][]dc.PortBinding{
			"22/tcp": {{HostPort: strconv.Itoa(sshPort)}},
		},
	})

	if err != nil {
		return nil, errors.Wrap(err, "error starting container")
	}

	p.log.Info("spew network settings", spew.Sdump(container.Container.NetworkSettings))

	p.log.Infof("%s address %s", container.Container.ID, container.GetBoundIP(""))
	p.container = container

	privateKey, publicKey, err := pkc.GenerateKeyPair()
	if err != nil {
		return nil, errors.Wrap(err, "error generating key pair")
	}
	p.log.Infof("private key: %s", privateKey)
	p.log.Infof("public key: %s", publicKey)
	p.privateKey = privateKey

	cmd = []string{"/bin/bash", "-c",
		fmt.Sprintf(`mkdir -p /root/.ssh/ && echo "%s %s %s" > /root/.ssh/authorized_keys`,
			"ssh-rsa", publicKey, container.Container.ID)}
	exit, err := container.Exec(cmd,
		dockertest.ExecOptions{
			Env:    []string{"DEBIAN_FRONTEND=noninteractive"},
			StdOut: os.Stdout,
			StdErr: os.Stderr,
		})
	if err != nil {
		return nil, errors.Wrapf(err, "error executing command %s in container", strings.Join(cmd, " "))
	}

	if exit != 0 {
		return nil, errors.Errorf("error starting sshd container %v, exit code %v",
			p.container.Container.ID, exit)
	}

	client, err := ssh.New(p.log, true, privateKey,
		p.Username(), "", fmt.Sprintf("127.0.0.1:%d", sshPort))
	if err != nil {
		return nil, errors.Wrap(err, "error creating ssh client")
	}

	p.ssh = client
	return client, nil
}

// Username provide the username for this provider. Each provider can specify a
// unique username. The docker username will be root.
func (p *ProviderBackend) Username() string {
	return "root"
}

// Password not used by this backend
func (p *ProviderBackend) Password() string {
	return ""
}

// PrivateKey returns the private key used for ssh auth
func (p *ProviderBackend) PrivateKey() []byte {
	return p.privateKey
}

// WithOption not used by this backend
func (p *ProviderBackend) WithOption(_ string, _ string) {
	// not implemented on this backend, does not use password auth
}
