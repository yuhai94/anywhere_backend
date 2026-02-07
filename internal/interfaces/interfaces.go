package interfaces

import (
	"context"

	"github.com/yuhai94/anywhere_backend/internal/aws"
	"github.com/yuhai94/anywhere_backend/internal/models"
)

type EC2ClientInterface interface {
	CreateInstance(ctx context.Context, region string, userData string, uuid string) (string, error)
	WaitForInstanceRunning(ctx context.Context, region string, instanceID string) error
	GetInstancePublicIP(ctx context.Context, region string, instanceID string) (string, error)
	TerminateInstance(ctx context.Context, region string, instanceID string) error
	DescribeInstances(ctx context.Context, region string) ([]aws.InstanceInfo, error)
	WaitForInstanceTerminated(ctx context.Context, region string, instanceID string) error
}

type RepositoryInterface interface {
	Create(ctx context.Context, instance *models.V2RayInstance) error
	GetByUUID(ctx context.Context, uuid string) (*models.V2RayInstance, error)
	List(ctx context.Context) ([]*models.V2RayInstance, error)
	Update(ctx context.Context, instance *models.V2RayInstance) error
	UpdateStatus(ctx context.Context, uuid string, status string) error
	UpdateStatusAndIP(ctx context.Context, uuid string, status string, publicIP string) error
	Delete(ctx context.Context, uuid string) error
	CheckRegionHasActiveInstance(ctx context.Context, region string) (bool, error)
	GetRegionActiveInstance(ctx context.Context, region string) (*models.V2RayInstance, error)
	LockTable(ctx context.Context) error
	UnlockTable(ctx context.Context) error
	InitSchema(ctx context.Context) error
}

type V2RayManagerInterface interface {
	AddInstance(ctx context.Context, instanceTag, address string, port int, uuid string) error
}
