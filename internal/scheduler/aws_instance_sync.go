package scheduler

import (
	"context"
	"time"

	"github.com/yuhai94/anywhere_backend/internal/aws"
	"github.com/yuhai94/anywhere_backend/internal/config"
	"github.com/yuhai94/anywhere_backend/internal/interfaces"
	"github.com/yuhai94/anywhere_backend/internal/logging"
	"github.com/yuhai94/anywhere_backend/internal/models"
)

// AWSInstanceSyncTask AWS实例同步任务
type AWSInstanceSyncTask struct {
	ec2Client interfaces.EC2ClientInterface
	repo      interfaces.RepositoryInterface
	ticker    *time.Ticker
	stopCh    chan struct{}
}

// NewAWSInstanceSyncTask 创建新的AWS实例同步任务
func NewAWSInstanceSyncTask(ec2Client interfaces.EC2ClientInterface, repo interfaces.RepositoryInterface) *AWSInstanceSyncTask {
	return &AWSInstanceSyncTask{
		ec2Client: ec2Client,
		repo:      repo,
		stopCh:    make(chan struct{}),
	}
}

// Name 返回任务名称
func (t *AWSInstanceSyncTask) Name() string {
	return "aws_instance_sync"
}

// Start 启动任务
func (t *AWSInstanceSyncTask) Start(ctx context.Context) {
	logging.Info(ctx, "Starting AWS instance sync task")

	// 立即执行一次同步
	t.syncInstances(ctx)

	// 设置定时器
	syncInterval := time.Duration(config.AppConfig.Scheduler.InstanceSyncInterval) * time.Second
	t.ticker = time.NewTicker(syncInterval)
	defer t.ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			logging.Info(ctx, "AWS instance sync task stopped due to context cancellation")
			return
		case <-t.stopCh:
			logging.Info(ctx, "AWS instance sync task stopped")
			return
		case <-t.ticker.C:
			t.syncInstances(ctx)
		}
	}
}

// Stop 停止任务
func (t *AWSInstanceSyncTask) Stop() {
	close(t.stopCh)
}

// syncInstances 同步AWS实例列表到数据库
func (t *AWSInstanceSyncTask) syncInstances(ctx context.Context) {
	logging.Info(ctx, "Starting AWS instance sync")

	// 从配置文件获取所有region
	regions := make([]string, 0, len(config.AppConfig.AWS.Regions))
	for region := range config.AppConfig.AWS.Regions {
		regions = append(regions, region)
	}

	// 获取数据库中的实例列表
	dbInstances, err := t.repo.List(ctx)
	if err != nil {
		logging.Error(ctx, "Failed to get instances from database: %v", err)
		return
	}

	// 创建数据库实例映射，用于快速查找
	dbInstanceMap := make(map[string]*models.V2RayInstance)
	for _, instance := range dbInstances {
		dbInstanceMap[instance.EC2ID] = instance
	}

	// 记录AWS中存在的实例ID
	awsInstanceIDs := make(map[string]bool)

	// 遍历每个region，获取实例列表
	for _, region := range regions {
		instances, err := t.ec2Client.DescribeInstances(ctx, region)
		if err != nil {
			logging.Error(ctx, "Failed to describe instances in region %s: %v", region, err)
			continue
		}

		for _, instance := range instances {
			awsInstanceIDs[instance.InstanceID] = true

			// 检查数据库中是否存在该实例
			if dbInstance, exists := dbInstanceMap[instance.InstanceID]; exists {
				// 数据库中存在，更新实例信息
				t.updateInstance(ctx, dbInstance, instance)
				delete(dbInstanceMap, instance.InstanceID)
			} else {
				// 数据库中不存在，创建新实例
				t.createInstance(ctx, instance)
			}
		}
	}

	// 数据库中存在但AWS中不存在的实例，标记为已删除
	for ec2ID, instance := range dbInstanceMap {
		logging.Info(ctx, "Instance %s not found in AWS, marking as deleted", ec2ID)
		if err := t.repo.Delete(ctx, instance.UUID); err != nil {
			logging.Error(ctx, "Failed to mark instance %s as deleted: %v", instance.UUID, err)
		}
	}

	logging.Info(ctx, "AWS instance sync completed")
}

// createInstance 创建新的实例记录
func (t *AWSInstanceSyncTask) createInstance(ctx context.Context, instance aws.InstanceInfo) {
	// 跳过没有UUID标签的实例
	if instance.UUID == "" {
		logging.Info(ctx, "Skipping instance %s without UUID tag", instance.InstanceID)
		return
	}

	newInstance := &models.V2RayInstance{
		UUID:        instance.UUID,
		EC2ID:       instance.InstanceID,
		EC2Region:   instance.Region,
		EC2PublicIP: instance.PublicIP,
		Status:      models.StatusRunning,
		IsDeleted:   false,
	}

	if err := t.repo.Create(ctx, newInstance); err != nil {
		logging.Error(ctx, "Failed to create instance record for %s: %v", instance.InstanceID, err)
	} else {
		logging.Info(ctx, "Created new instance record for %s with ID: %d", instance.InstanceID, newInstance.ID)
	}
}

// updateInstance 更新实例记录
func (t *AWSInstanceSyncTask) updateInstance(ctx context.Context, dbInstance *models.V2RayInstance, instance aws.InstanceInfo) {
	// 更新公网IP
	if dbInstance.EC2PublicIP != instance.PublicIP {
		dbInstance.EC2PublicIP = instance.PublicIP
		logging.Info(ctx, "Updated public IP for instance %s from %s to %s", instance.InstanceID, dbInstance.EC2PublicIP, instance.PublicIP)
	}

	// 更新状态
	if dbInstance.Status != instance.Status {
		dbInstance.Status = instance.Status
		logging.Info(ctx, "Updated status for instance %s to %s", instance.InstanceID, instance.Status)
	}

	if err := t.repo.Update(ctx, dbInstance); err != nil {
		logging.Error(ctx, "Failed to update instance record for %s: %v", instance.InstanceID, err)
	}
}
