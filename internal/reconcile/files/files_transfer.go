package files

import (
	"bytes"
	"crypto/sha256"
	"fmt"
	"io"
	"os"
	"path"

	"github.com/pkg/errors"

	"zappem.net/pub/debug/xxd"

	"slack-reconcile-deployments/internal/manifest"
)

// Transfer transfers one file to remote system
func (fm *FileManager) Transfer(f *manifest.File, reader io.Reader) (bool, error) {
	// set differences to false, if we find modified files, we'll set it to true
	if fm.scp == nil {
		return false, errors.New("error: scp client not initialized")
	}

	stat, err := fm.Stat(f)
	if err != nil {
		if !os.IsNotExist(err) {
			return false, errors.Wrapf(err, "error stat %s", f.Path)
		}
	}
	if stat != nil {
		fm.log.Infof("stat %s: %+v", f.Path, stat)
	} else {
		fm.log.Infof("warning: stat returned nil for %s", f.Path)
	}

	b, err := io.ReadAll(reader)
	if err != nil {
		return false, errors.Wrapf(err, "error reading file %s", f.Path)
	}

	// only check shas when the remove file exists
	shalocal := ""
	if stat != nil {
		// remote sha is already in hex
		shalocal = fmt.Sprintf("%x", sha256.Sum256(b))
		fm.log.Infof("sha256 %s, local: %s, remote: %s", f.Path, shalocal, stat.Sha256)
		fm.log.Infof("local file %s", b)
	}

	baseName := path.Base(f.Path)
	tmpName := path.Join("/", "tmp", baseName)
	fm.log.Infof("writing %s, len: %d, to %s", f.Content, len(b), tmpName)

	if stat != nil && shalocal == stat.Sha256 {
		fm.log.Infof("not transferring file %s, no differnces detected", f.Path)
		return false, nil
	}

	if err := fm.scp.Copy(reader, tmpName); err != nil {
		return false, errors.Wrap(err, "error copying file")
	}

	// re-read transferred file to verify contents are there.
	// this is useful for debugging, but overkill normally.
	out, err := fm.ssh.Execf(fmt.Sprintf("/bin/cat %s", tmpName))
	if err != nil {
		return false, errors.Wrapf(err, "error exec cat %s", tmpName)
	}
	fm.log.Infof("contents read from remote file %s: %s", tmpName, out)

	// generate a hex dump of the contents read from remote. Useful for debugging.
	w := bytes.NewBuffer([]byte{})
	xxd.Print(w, 0x0, out)
	fm.log.Infof("hex dump of remote file %s: %s", tmpName, w.String())

	// move file to its intended location, from tmp
	// we use /tmp if the ssh user does not have write access to the
	// destination directory, move using sudo.
	out, err = fm.ssh.Execf(fmt.Sprintf("mv %s %s", tmpName, f.Path))
	if err != nil {
		return false, errors.Wrapf(err, "error exec cat %s", f.Path)
	}

	// TODO: set owner and file modes
	fm.log.Infof("moved %s to %s, out: '%s'", tmpName, f.Path, out)
	return true, nil
}
