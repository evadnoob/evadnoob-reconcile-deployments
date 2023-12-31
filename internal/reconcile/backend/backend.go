package backend

import (
	"context"

	"slack-reconcile-deployments/internal/ssh"
)

// ProviderBackendReconciler is the interface for reconciling deployments
type ProviderBackendReconciler interface {
	// Run runs the reconciler
	Run(ctx context.Context) (*ssh.Client, error)
	// Close to clean up resources created on or by the backend
	Close()
	// Username used to connect over ssh
	Username() string
	// Password used to connect over ssh
	Password() string
	// WithOption sets an option on the backend
	WithOption(name string, value string)
}
