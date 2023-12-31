package files

import (
	"fmt"

	"github.com/pkg/errors"
)

// ApplyPermissions applies permissions to files on remote system over ssh
func (fm *FileManager) ApplyPermissions() error {
	if fm.ssh == nil {
		return errors.New("error: ssh client not initialized")
	}
	for _, pkgs := range fm.manifest.Packages {
		for _, f := range pkgs.Files {
			if f.Path == "" {
				fm.log.Infof("skipping empty filepath on file: %+v", f)
				continue
			}
			if f.Mode != "" {
				out, err := fm.ssh.Execf(fmt.Sprintf("chmod %s %s", f.Mode, f.Path))
				if err != nil {
					return errors.Wrapf(err, "error exec chmod %s", f.Path)
				}
				fm.log.Infof("chmod %s %s, out: '%s'", f.Mode, f.Path, out)
			}

			if f.Owner != "" {
				out, err := fm.ssh.Execf(fmt.Sprintf("chown %s %s", f.Owner, f.Path))
				if err != nil {
					return errors.Wrapf(err, "error exec chown %s", f.Path)
				}
				fm.log.Infof("chown %s %s, out: '%s'", f.Mode, f.Path, out)
			}
		}
	}
	return nil
}
