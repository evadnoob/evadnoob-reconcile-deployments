package files

import (
	"embed"

	"go.uber.org/zap"

	"slack-reconcile-deployments/internal/manifest"
	"slack-reconcile-deployments/internal/ssh"
)

//go:embed templates/*
var fs embed.FS

const (
	// FileTemplateKeyLastModifiedDate key for the modified date in rendered templates
	FileTemplateKeyLastModifiedDate = "LastModifiedDate"
)

// FileManager manages files on remote systems
type FileManager struct {
	// log is the logger
	log *zap.SugaredLogger
	// manifest is the manifest
	manifest *manifest.Manifest
	// ssh client to invoke package commands on
	ssh *ssh.Client
	// scp lazily created when ssh client is provided
	scp *ssh.SecureCopyClient
}

// New creates a new files object to manage files on remote systems.
func New(log *zap.SugaredLogger, m *manifest.Manifest, sshClient *ssh.Client) *FileManager {
	return &FileManager{
		log:      log,
		manifest: m,
		ssh:      sshClient,
	}
}
