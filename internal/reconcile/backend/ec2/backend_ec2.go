package ec2

import (
	"context"
	"fmt"
	"os"
	"time"

	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/cenkalti/backoff/v4"
	"github.com/pkg/errors"
	"go.uber.org/zap"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/ec2"
	"github.com/aws/aws-sdk-go-v2/service/ec2/types"

	"slack-reconcile-deployments/internal/manifest"
	"slack-reconcile-deployments/internal/reconcile/backend"
	"slack-reconcile-deployments/internal/ssh"
)

// ErrNoInstanceFound indicates no instance found for when filtering by tags for ec2 instance.
var ErrNoInstanceFound = errors.New("no instance found")

// verify backend implements interface for backends
var _ backend.ProviderBackendReconciler = &ProviderBackend{}

// ProviderBackend is the ec2 specific provider backend
type ProviderBackend struct {
	log      *zap.SugaredLogger
	Client   *ec2.Client
	Manifest *manifest.Manifest
	// ssh client is lazily loaded once backend is running
	ssh           *ssh.Client
	PrivateKey    []byte
	HostID        string
	PublicDNSName string
}

// New creates a new provider backend.
// Dependencies on ec2 and the desired manifest are expected.
func New(log *zap.SugaredLogger, ctx context.Context,
	manifest *manifest.Manifest) (backend.ProviderBackendReconciler, error) {
	cfg, err := config.LoadDefaultConfig(ctx)
	if err != nil {
		return nil, errors.Wrap(err, "failed to load default configuration")
	}

	privateKeyPath := manifest.Parameters["private-key-path"]
	if _, err := os.Stat(privateKeyPath); os.IsNotExist(err) {
		return nil, errors.Errorf("private key file %s does not exist", privateKeyPath)
	}
	if err != nil {
		return nil, errors.Wrapf(err, "error reading private key file %s", privateKeyPath)
	}

	log.Info("creating ec2 client")
	client := ec2.NewFromConfig(cfg)

	privateKey, err := os.ReadFile(privateKeyPath)
	if err != nil {
		return nil, errors.Wrapf(err, "error reading private key file %s", privateKeyPath)
	}

	return &ProviderBackend{
		log:        log,
		Client:     client,
		Manifest:   manifest,
		PrivateKey: privateKey,
	}, nil
}

// Run reconciles provider state with desired state
func (p *ProviderBackend) Run(ctx context.Context) (*ssh.Client, error) {
	// when exists, move on, we'll wait for running state later
	instanceID, publicDNSName, err := p.reconcileEC2(ctx)
	if err != nil {
		return nil, errors.Wrap(err, "error reconciling ec2")
	}
	p.HostID = instanceID
	p.PublicDNSName = publicDNSName
	p.log.Infof("instance %s, %s, %s exists", instanceID, p.Manifest.ID, publicDNSName)

	p.ssh, err = ssh.New(p.log, true, p.PrivateKey,
		p.Username(), "", fmt.Sprintf("%s:22", publicDNSName))
	if err != nil {
		return nil, errors.Wrap(err, "error creating ssh client")
	}
	return p.ssh, nil
}

// WaitForRunning waits until an instance exists and is in running state
func (p *ProviderBackend) WaitForRunning(ctx context.Context, instanceID string) error {
	if err := backoff.Retry(func() error {
		out, err := p.Client.DescribeInstances(ctx,
			&ec2.DescribeInstancesInput{InstanceIds: []string{instanceID}})
		if err != nil {
			return err
		}
		// when reservations > 0, this is good, we found an instance
		if len(out.Reservations) > 0 {
			for _, r := range out.Reservations {
				// really we expect one instance here
				instanceID := *r.Instances[0].InstanceId
				instanceStateName := *r.Instances[0].State
				p.PublicDNSName = *r.Instances[0].PublicDnsName
				p.log.Infof("checking instance state instanceId: %v, instanceState: %v", instanceID, instanceStateName)
				if r.Instances[0].State.Name != types.InstanceStateNameRunning {
					p.log.Infof("not running, instance found for %v", instanceID)
					return errors.Errorf("intance found, state: %v %v", instanceID, instanceStateName)
				}
			}
			return nil
		}
		return ErrNoInstanceFound
	}, backoff.WithMaxRetries(backoff.NewConstantBackOff(10*time.Second), 15)); err != nil {
		return err
	}
	p.log.Infof("instance is running %s, %s, %s", instanceID, p.PublicDNSName, p.Manifest.ID)
	return nil
}

// Exists checks for an instance with given name
// TODO: add context timeout/stop
func (p *ProviderBackend) Exists(ctx context.Context, name string) (bool, string, string, error) {
	exists := false
	instanceID := ""
	publicDNSName := ""

	if err := backoff.Retry(func() error {
		out, err := p.Client.DescribeInstances(ctx,
			&ec2.DescribeInstancesInput{Filters: []types.Filter{
				{Name: aws.String("tag:Name"), Values: []string{name}},
			}})
		if err != nil {
			p.log.Warnf("%v", err)
			return err
		}
		// when reservations > 0, this is good, we found an instance
		if len(out.Reservations) > 0 {
			for _, r := range out.Reservations {
				if len(r.Instances) > 0 {
					// skip terminated instances, they'll be cleaned up later
					if r.Instances[0].State.Name == types.InstanceStateNameTerminated {
						p.log.Infof("instance %s is terminated", *r.Instances[0].InstanceId)
						continue
					}
				}
				// instance exists, be done with the retries
				exists = true
				instanceID = *r.Instances[0].InstanceId
				publicDNSName = *r.Instances[0].PublicDnsName
				return nil
			}
		}
		return ErrNoInstanceFound
	}, backoff.WithMaxRetries(backoff.NewConstantBackOff(5*time.Second), 2)); err != nil {
		return false, "", "", err
	}
	return exists, instanceID, publicDNSName, nil
}

// Close closes the ssh client and session
func (p *ProviderBackend) Close() {
	if p.ssh != nil {
		p.ssh.Close()
	}
}

// reconcileEC2 creates an ec2 instance
func (p *ProviderBackend) reconcileEC2(ctx context.Context) (string, string, error) {
	exists, instanceID, publicDNSName, err := p.Exists(ctx, p.Manifest.ID)
	if err != nil {
		if !errors.Is(err, ErrNoInstanceFound) {
			return "", "", errors.Wrapf(err, "error checking for instance %s", p.Manifest.ID)
		}
	}

	if exists {
		p.log.Infof("instance %s exists, %s", instanceID, publicDNSName)
		return instanceID, publicDNSName, nil
	}

	out, err := p.Client.RunInstances(ctx, &ec2.RunInstancesInput{
		MaxCount: aws.Int32(1),
		MinCount: aws.Int32(1),
		BlockDeviceMappings: []types.BlockDeviceMapping{{
			DeviceName: aws.String("/dev/sdh"),
			Ebs: &types.EbsBlockDevice{
				DeleteOnTermination: aws.Bool(true),
				VolumeSize:          aws.Int32(8),
				VolumeType:          types.VolumeTypeGp3,
			},
		}},
		ImageId:                           aws.String(p.Manifest.Parameters["image-id"]),
		InstanceInitiatedShutdownBehavior: types.ShutdownBehaviorTerminate,
		InstanceType:                      types.InstanceType(p.Manifest.Parameters["size"]),
		KeyName:                           aws.String(p.Manifest.Parameters["key-name"]),
		// Only currently allowing one security group id.
		SecurityGroupIds: []string{p.Manifest.Parameters["security-group-id"]},
		SubnetId:         aws.String(p.Manifest.Parameters["subnet-id"]),
		TagSpecifications: []types.TagSpecification{{
			ResourceType: types.ResourceTypeInstance,
			Tags: []types.Tag{
				{Key: aws.String("Name"), Value: aws.String(p.Manifest.ID)},
			}}},
		UserData: nil,
	})
	if err != nil {
		return "", "", errors.Wrapf(err, "error creating instance %s", p.Manifest.ID)
	}
	p.log.Infof("created instance %+v", out)
	instanceID = *out.Instances[0].InstanceId
	p.HostID = instanceID

	p.log.Infof("waiting for instance %s to be running %v", p.HostID, p.Manifest.ID)
	if err := p.WaitForRunning(ctx, *out.Instances[0].InstanceId); err != nil {
		return "", "", errors.Wrapf(err, "error waiting for instance %s", p.Manifest.ID)
	}
	p.log.Infof("instance %s is running", instanceID)
	return *out.Instances[0].InstanceId, p.PublicDNSName, nil
}

// Username provide the username for this provider. Each provider can specify a
// unique username. The ec2 username will be admin.
func (p *ProviderBackend) Username() string {
	return "admin"
}

// Password not used by this backend
func (p *ProviderBackend) Password() string {
	return ""
}

// WithOption not used by this backend
func (p *ProviderBackend) WithOption(_ string, _ string) {
	// not implemented on this backend, does not use password auth
}
