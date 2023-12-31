package files

import (
	"bytes"
	"io"
	"strings"
	"text/template"

	"github.com/pkg/errors"

	"slack-reconcile-deployments/internal/manifest"
	"slack-reconcile-deployments/internal/ssh"
)

// RenderAndTransfer renders and transfers all files
func (fm *FileManager) RenderAndTransfer(data map[string]string) (map[string]manifest.Package, error) {
	if fm.ssh == nil {
		return nil, errors.New("error: ssh client not initialized")
	}
	changedPackages := make(map[string]manifest.Package)
	fm.scp = ssh.NewSecureCopyClient(fm.log, fm.ssh)
	for _, pkg := range fm.manifest.Packages {
		for _, f := range pkg.Files {
			if f.Path == "" {
				return nil, errors.New("error: file path not set")
			}
			reader, err := fm.Render(&pkg, &f, data)
			if err != nil {
				return nil, errors.Wrapf(err, "error rendering %s", f.Path)
			}
			differences, err := fm.Transfer(&f, reader)
			if err != nil {
				return changedPackages, errors.Wrapf(err, "error transferring %s", f.Path)
			}
			if differences {
				fm.log.Infof("differences detected for file: %s, package: %s", f.Path, pkg)
				changedPackages[pkg.Name] = pkg
			}
		}
	}
	return changedPackages, nil
}

// Render renders one files using templates
func (fm *FileManager) Render(p *manifest.Package, f *manifest.File, data map[string]string) (io.Reader, error) {
	filename := f.Path
	var reader io.Reader
	// support static content directly read from manifest/files
	// also support embedded files for larger content.
	if strings.HasPrefix(f.Content, "embed://") {
		// read template
		filenameToRead := strings.TrimPrefix(f.Content, "embed://")
		b1, err := fs.ReadFile(filenameToRead)
		if err != nil {
			return nil, errors.Wrapf(err, "error reading file %s", filenameToRead)
		}

		tmpl, err := template.New(filename).Parse(string(b1))
		if err != nil {
			return nil, errors.Wrapf(err, "error parsing template %s", filenameToRead)
		}

		// Build a map of data to provide to template
		merged := make(map[string]string)
		for k, v := range data {
			merged[k] = v
		}
		for k, v := range p.Parameters {
			merged[k] = v
		}
		// merge data + package metadata
		merged["Version"] = p.Version

		// render template
		b2 := bytes.NewBuffer([]byte{})
		err = tmpl.Execute(b2, merged)
		if err != nil {
			return nil, errors.Wrapf(err, "error executing template %s", filenameToRead)
		}

		// setup reader for rendered bytes
		reader = bytes.NewReader(b2.Bytes())
	} else {
		reader = bytes.NewReader([]byte(f.Content))
	}
	return reader, nil

}
