package ssh

import (
	"fmt"
	"path"
	"time"

	"github.com/cenkalti/backoff/v4"
	"github.com/davecgh/go-spew/spew"
	"github.com/pkg/errors"
	"go.uber.org/zap"
	"golang.org/x/crypto/ssh"
	"golang.org/x/crypto/ssh/knownhosts"

	"slack-reconcile-deployments/internal/homedir"
)

// Client is an ssh client
type Client struct {
	log     *zap.SugaredLogger
	client  *ssh.Client
	host    string
	useSudo bool
}

// New creates a new ssh session.
//
// Tests require running sshd in some way. The following will start a sshd server
// with root login permitted. Doing this for testing only.
//
// start sshd in container:
//
//	docker run -it -p 22:22 debian:12.2 /bin/bash  -c 'apt-get update && apt-get install -y ca-certificates \
//	   openssh-client openssh-server && mkdir /run/sshd && /usr/sbin/sshd -D -e -o IgnoreUserKnownHosts=yes -o \
//		  PermitEmptyPasswords=yes -o PermitRootLogin=yes'
//
//		send public key to authorized_keys:
//		 docker exec -it d9a4db472e2c  /bin/bash -c \
//		   'mkdir -p /root/.ssh/ && echo "$(cat ~/.ssh/id_rds)" > /root/.ssh/authorized_keys'
//
// connect client to sshd ignoring known hosts:
//
// ssh -o StrictHostKeyChecking=no root@127.0.0.1
func New(log *zap.SugaredLogger, allowInsecureHostKey bool,
	privateKey []byte, username, password, host string) (*Client, error) {
	log.Infof("dialing %s@%s", username, host)
	var err error
	var signer ssh.Signer
	if len(privateKey) > 0 {
		signer, err = ssh.ParsePrivateKey(privateKey)
		if err != nil {
			return nil, errors.Wrap(err, "unable to parse private key")
		}
	}
	// host key callbacks allowed: insecure, fixed, or a local(.ssh/known_hosts).
	// use the current users known host key by default
	hostKeyCallback := ssh.InsecureIgnoreHostKey()
	if !allowInsecureHostKey {
		userHomeDir := homedir.Get()
		hostKeyCallback, err = knownhosts.New(path.Join(userHomeDir, ".ssh", "known_hosts"))
		if err != nil {
			return nil, errors.Wrapf(err, "error on known hosts")
		}
	}

	// setup auth methods, will use password as fallback if private/public keys not available.
	// only append one auth method because only the first method provided will be used.
	authMethods := make([]ssh.AuthMethod, 0, 2)
	if signer != nil {
		authMethods = append(authMethods, ssh.PublicKeys(signer))
	}
	if password != "" {
		authMethods = append(authMethods, ssh.Password(password))
	}
	log.Infof("auth methods %s", spew.Sdump(authMethods))

	config := &ssh.ClientConfig{
		User:            username,
		Auth:            authMethods,
		HostKeyCallback: hostKeyCallback,
	}

	var client *ssh.Client
	if err := backoff.Retry(func() error {
		client, err = ssh.Dial("tcp", host, config)
		if err != nil {
			log.Infof("error dialing %s@%s, retrying %+v", username, host, err)
			return errors.Wrap(err, "failed to dial")
		}
		return nil
	}, backoff.WithMaxRetries(backoff.NewConstantBackOff(3*time.Second), 30)); err != nil {
		return nil, errors.Wrap(err, "retries exhausted, ssh, failed to dial")
	}

	log.Infof("server version %s, client version %s", client.ServerVersion(), client.ClientVersion())

	return &Client{
		log:     log,
		host:    host,
		client:  client,
		useSudo: username != "root",
	}, nil

}

// Execf executes a command on the ssh session
func (c *Client) Execf(cmd string, args ...interface{}) ([]byte, error) {
	session, err := c.client.NewSession()
	if err != nil {
		return nil, errors.Wrap(err, "failed to create session")
	}
	defer func() {
		_ = session.Close()
	}()

	// replace format spec with args
	cmd = fmt.Sprintf(cmd, args...)
	if c.useSudo {
		cmd = fmt.Sprintf("sudo %s", cmd)
	}
	c.log.Infof("exec %s", cmd)
	buf, err := session.CombinedOutput(cmd) // cmd is ignored by fixedOutputHandler
	if err != nil {
		return nil, errors.Wrapf(err, "remote command did not exit cleanly: %s", buf)
	}

	return buf, err
}

// Close closes the ssh client and session
func (c *Client) Close() {
	if err := c.client.Close(); err != nil {
		c.log.Warnf("failed to close client: %v", err)
	}
}

// PublicKeyFromPrivateKey returns the public key from a private key
func PublicKeyFromPrivateKey(privateKey []byte) (ssh.PublicKey, error) {
	signer, err := ssh.ParsePrivateKey(privateKey)
	if err != nil {
		return nil, errors.Wrap(err, "unable to parse private key")
	}
	return signer.PublicKey(), nil
}
