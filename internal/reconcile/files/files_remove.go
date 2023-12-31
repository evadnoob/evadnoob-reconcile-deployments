package files

import (
	"github.com/pkg/errors"

	"slack-reconcile-deployments/internal/manifest"
)

// Remove removes files for packages
func (fm *FileManager) Remove(pkgs ...manifest.Package) error {
	for _, pkg := range pkgs {
		for _, f := range pkg.Files {
			if f.Path == "" {
				return errors.New("error: file path not set")
			}
			if err := fm.RemoveOne(&f); err != nil {
				fm.log.Infof("error removing file %s", f.Path)
			}
		}
	}
	return nil
}

// RemoveOne removes one file
func (fm *FileManager) RemoveOne(f *manifest.File) error {
	out, err := fm.ssh.Execf("rm -f %s", f.Path)
	if err != nil {
		fm.log.Infof("warning: error stat %s: %s", f.Path, err)
		return nil // ignore error, file may not exist
	}
	fm.log.Infof("removed file %s, out: %s", f.Path, out)
	return nil
}
