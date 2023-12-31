package testhelpers

import (
	"fmt"
	"os"
	"strconv"
	"testing"

	"github.com/davecgh/go-spew/spew"
	"github.com/ory/dockertest/v3"
	dc "github.com/ory/dockertest/v3/docker"
	"go.uber.org/zap"
	"gotest.tools/v3/assert"

	"slack-reconcile-deployments/internal/freeport"
	"slack-reconcile-deployments/internal/logging"
	"slack-reconcile-deployments/internal/pkc"
)

// DockerTestFixtures is a test fixture for ssh tests with docker
type DockerTestFixtures struct {
	Log        *zap.SugaredLogger
	PrivateKey []byte
	PublicKey  []byte
	SSHPort    int
}

// NewDockerTestFixtures creates a new DockerSSHTestFixtures
func NewDockerTestFixtures(t *testing.T) *DockerTestFixtures {
	return &DockerTestFixtures{Log: logging.New(t.Name(), false)}
}

// Run runs the docker ssh test fixtures, non-blocking
func (s *DockerTestFixtures) Run(t *testing.T) {
	pool, err := dockertest.NewPool("")
	assert.NilError(t, err, "new pool")

	err = pool.Client.Ping()
	assert.NilError(t, err, "ping")

	cmd := []string{"/bin/bash", "-c", `apt-get update && apt-get install -y ca-certificates \
		 openssh-client openssh-server && mkdir /run/sshd && /usr/sbin/sshd -D -e -o \
         IgnoreUserKnownHosts=yes -o PermitEmptyPasswords=yes -o PermitRootLogin=yes`}

	sshPort, err := freeport.GetFreePort()
	assert.NilError(t, err, "get free port")

	container, err := pool.RunWithOptions(&dockertest.RunOptions{
		Repository:   "debian",
		Tag:          "12.2",
		Cmd:          cmd,
		ExposedPorts: []string{"22/tcp"},
		PortBindings: map[dc.Port][]dc.PortBinding{
			"22/tcp": {{HostPort: strconv.Itoa(sshPort)}},
		},
	})

	assert.NilError(t, err, "run container")
	assert.Equal(t, strconv.Itoa(sshPort), container.GetPort("22/tcp"))

	t.Cleanup(func() {
		if container == nil {
			return
		}
		if err := container.Expire(1); err != nil {
			t.Logf("error expiring container %v %v", container, err)
		}
		if err := container.Close(); err != nil {
			t.Logf("error stopping container %v %v", container, err)
		}
	})
	spew.Dump(container.Container.NetworkSettings)

	t.Logf("%s address %s", container.Container.ID, container.GetBoundIP(""))

	privateKey, publicKey, err := pkc.GenerateKeyPair()
	assert.NilError(t, err, "generate key pair")
	// normally would not log private key, but for testing purposes, and these are
	// dynamically created, local to docker
	t.Logf("private key: %s", privateKey)
	t.Logf("public key: %s", publicKey)

	cmd = []string{"/bin/bash", "-c",
		fmt.Sprintf(`mkdir -p /root/.ssh/ && echo "%s %s %s" > /root/.ssh/authorized_keys`,
			"ssh-rsa", publicKey, container.Container.ID)}
	exit, err := container.Exec(cmd,
		dockertest.ExecOptions{
			Env:    []string{"DEBIAN_FRONTEND=noninteractive"},
			StdOut: os.Stdout,
			StdErr: os.Stderr,
		})
	assert.NilError(t, err, "exec authorized_keys")
	assert.Equal(t, exit, 0, "exit authorized_keys")

	s.SSHPort = sshPort
	s.PrivateKey = privateKey
	s.PublicKey = publicKey
}
