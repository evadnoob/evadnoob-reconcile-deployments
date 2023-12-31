package generate

import (
	"embed"
	"io"
	"path"
	"text/template"

	"github.com/pkg/errors"

	"slack-reconcile-deployments/internal/manifest"
)

//go:embed templates/*
var fs embed.FS

// Run generate new manifests to help bootstrap new deployments
// use format to select between the default id format, random bytes in hex, or
// ulid, a lexicographically sortable id.
func Run(writer io.Writer, format manifest.UniqueIDFormat) error {
	direntries, err := fs.ReadDir("templates")
	if err != nil {
		return errors.Wrapf(err, "error reading directory")
	}
	for _, direntry := range direntries {
		b1, err := fs.ReadFile(path.Join("templates", direntry.Name()))
		if err != nil {
			return errors.Wrapf(err, "error reading file %s", direntry.Name())
		}
		tmpl, err := template.New(direntry.Name()).Parse(string(b1))
		if err != nil {
			return errors.Wrapf(err, "error parsing template %s", direntry.Name())
		}
		id, err := manifest.NewID(format)
		if err != nil {
			return errors.Wrapf(err, "error generating unique manifest id")
		}
		data := map[string]string{
			"ID": id,
		}
		if err := tmpl.Execute(writer, data); err != nil {
			return errors.Wrapf(err, "error executing template %s", direntry.Name())
		}
		_, _ = writer.Write([]byte("\n"))
	}
	return nil
}
