package ssh

import (
	"bufio"
	"io"
	"os"
	"path"

	"github.com/pkg/errors"
	"github.com/pkg/sftp"
	"go.uber.org/zap"
	"golang.org/x/crypto/ssh"
)

// SecureCopyClient is a client that copies data to remote systems over ssh.
type SecureCopyClient struct {
	// log is the logger
	log *zap.SugaredLogger
	// ssh client to invoke package commands on
	ssh *ssh.Client
}

// NewSecureCopyClient creates a new secure copy client.
// Use the secure copy client to copy over an existing ssh connection.
func NewSecureCopyClient(log *zap.SugaredLogger, client *Client) *SecureCopyClient {
	return &SecureCopyClient{
		log: log,
		ssh: client.client,
	}
}

// Copy copies bytes from local system to remote system writing bytes to the
// filepath provided.
//
// Copy will attempt to make the parent directories of the file, and ignore any
// exists errors.
//
// MaxPacket size is 1<<15(32kb) so we don't set that option.
func (s *SecureCopyClient) Copy(data io.Reader, filepath string) error {
	c, err := sftp.NewClient(s.ssh)
	if err != nil {
		return errors.Wrap(err, "unable to start sftp from ssh client")
	}
	defer func() {
		_ = c.Close()
	}()

	// reset the reader back to the beginning, to ensure we're copying from beginning
	if seeker, ok := data.(io.Seeker); ok {
		_, err := seeker.Seek(0, io.SeekStart)
		if err != nil {
			return errors.Wrapf(err, "error seeking reader for %s", filepath)
		}
	}

	// make the directories for the file. Use MkdirAll to ignore existing
	// parent directories.
	if err := c.MkdirAll(path.Dir(filepath)); err != nil {
		if !os.IsExist(err) {
			return errors.Wrapf(err,
				"error creating directory %s", path.Dir(filepath))
		}
	}

	w, err := c.OpenFile(filepath, os.O_WRONLY|os.O_CREATE|os.O_TRUNC)
	if err != nil {
		return errors.Wrap(err, "error opening remote file for writing")
	}
	defer func() {
		_ = w.Close()
	}()

	writer := bufio.NewWriter(w)
	defer func() {
		if err := writer.Flush(); err != nil {
			s.log.Warnf("error flushing writer: %v", err)
		}
	}()

	// make byte buffer of (1 * 2^10) 1kb for the reader
	buf := make([]byte, 1<<10)
	for {
		n, err := data.Read(buf)
		if n > 0 {
			s.log.Infof("read %d bytes", n)
			nn, err2 := writer.Write(buf[:n])
			if err2 != nil {
				return errors.Wrapf(err2, "error writing to remote file while copying %s", filepath)
			}
			s.log.Infof("wrote %d bytes", nn)
		}
		if err != nil {
			if err == io.EOF {
				break
			}
			return errors.Wrapf(err, "error reading from reader while copying %s", filepath)
		}

		if err != nil {
			return errors.Wrapf(err, "error writing to remote file while copying %s", filepath)
		}

	}
	return nil
}
