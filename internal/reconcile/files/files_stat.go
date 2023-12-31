package files

import (
	"bytes"
	"fmt"
	"os"
	"strconv"
	"strings"

	"github.com/pkg/errors"
	cryptossh "golang.org/x/crypto/ssh"

	"slack-reconcile-deployments/internal/manifest"
)

// Stat is the result of a stat command.
// See Stat() for more details.
type Stat struct {
	// Name of the file
	Name string
	// Type of the object, symlink, regular file, etc.
	Type string
	// Size in bytes of the object, will be -1 if not able to parse
	Size int64
	// Owner is the user owner of the object
	Owner string
	// Group is the group owner of the object
	Group string
	// LastModifiedTime is the timestamp the last time the file was modified
	LastModifiedTime string
	// Sha256 is the sha256 hash of the file, as a hex string
	Sha256 string
}

// Stat emulates the stat linux command
//
// stat -c '%n,%F,%s,%U,%G,%y' filename
// xyz,regular empty file,0,root,root,2023-12-14 19:32:05.044939009 +0000
//
// Format specifiers(see man stat):
//
//		%n file name
//		%F file type
//		%s total size, in bytes
//		%U username of owner
//		%G group name of owner
//	 %y time of last data modification, human-readable
func (fm *FileManager) Stat(f *manifest.File) (*Stat, error) {
	if f == nil {
		return nil, errors.New("error: file cannot be nil")
	}
	// use %% to escape % in format string
	out, err := fm.ssh.Execf("stat -c '%%n,%%F,%%s,%%U,%%G,%%y' %s", f.Path)
	if err != nil {
		fm.log.Infof("warning: error stat %s: %s", f.Path, err)
		var exitErr *cryptossh.ExitError
		if errors.As(err, &exitErr) {
			if strings.Contains(err.Error(), "No such file or directory") {
				return nil, os.ErrNotExist
			}
		}
		// file may not exist, caller must handle ssh.ExitStatus
		return nil, err
	}
	fm.log.Infof("stat %s: %s", f.Path, strings.TrimSpace(string(out)))
	parts := strings.Split(strings.TrimSpace(string(out)), ",")
	if len(parts) != 6 {
		return nil, errors.Errorf("error parsing stat output: %s", out)
	}
	size, err := strconv.ParseInt(parts[2], 10, 64)
	if err != nil {
		size = int64(-1)
	}
	stat := Stat{
		Name:             parts[0],
		Type:             parts[1],
		Size:             size,
		Owner:            parts[3],
		Group:            parts[4],
		LastModifiedTime: parts[5],
	}

	// get a sha 256 for the file, can be used to detect differences.
	// Example:
	// sha256sum  /etc/os-release
	// 98fa979a2418b2a4e8789a8dc6c8c2ce3c9b0e3c8210658bb2d16956c125dcce  /etc/os-release
	out, err = fm.ssh.Execf(fmt.Sprintf("sha256sum %s |awk '{ print $1 }'", f.Path))
	if err != nil {
		return nil, errors.Wrapf(err, "error stat %s", f.Path)
	}
	out = bytes.TrimSpace(out)
	fm.log.Infof("stat %s: out: '%s'", f.Path, out)
	if len(out) == 0 {
		fm.log.Infof("warning: parsing sha256 for %s, output: %s", out, f.Path)
	}
	stat.Sha256 = string(out)
	// remove the prefix, for comparing to hex shas
	stat.Sha256 = strings.TrimPrefix(stat.Sha256, "Sha256:")
	return &stat, nil
}
