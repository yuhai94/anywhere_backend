package service

import (
	"context"
	"encoding/base64"
	"fmt"
	"strings"
	"sync"

	"github.com/google/uuid"
	"github.com/yuhai94/anywhere_backend/internal/aws"
	"github.com/yuhai94/anywhere_backend/internal/config"
	"github.com/yuhai94/anywhere_backend/internal/localv2ray"
	"github.com/yuhai94/anywhere_backend/internal/logging"
	"github.com/yuhai94/anywhere_backend/internal/models"
	"github.com/yuhai94/anywhere_backend/internal/repository"
)

type V2RayService struct {
	repo              *repository.Repository
	ec2Client         *aws.EC2Client
	localV2RayManager *localv2ray.LocalV2RayManager
	wg                sync.WaitGroup
}

// NewV2RayService 创建一个新的 V2RayService 实例
// 参数:
//   - repo: Repository 实例，用于数据库操作
//   - ec2Client: EC2Client 实例，用于 AWS EC2 操作
//
// 返回值:
//   - *V2RayService: 新创建的 V2RayService 实例
//
// 功能:
//  1. 初始化 V2RayService 结构体
//  2. 如果配置了本地 V2Ray 配置路径，创建 LocalV2RayManager 实例
//  3. 返回配置好的 V2RayService 实例
func NewV2RayService(repo *repository.Repository, ec2Client *aws.EC2Client) *V2RayService {
	var localV2RayManager *localv2ray.LocalV2RayManager
	if config.AppConfig.V2Ray.LocalConfigPath != "" {
		localV2RayManager = localv2ray.NewLocalV2RayManager(config.AppConfig.V2Ray.LocalConfigPath)
	}

	return &V2RayService{
		repo:              repo,
		ec2Client:         ec2Client,
		localV2RayManager: localV2RayManager,
	}
}

// CreateInstance 创建 V2Ray 实例
// 参数:
//   - ctx: 上下文，用于传递请求范围的值
//   - region: AWS 区域
//
// 返回值:
//   - string: 实例 UUID
//   - error: 错误信息，如果操作失败
//
// 功能:
//  1. 检查指定region是否已有活跃实例
//  2. 如果已有活跃实例，返回该实例的UUID
//  3. 如果没有，生成实例 UUID
//  4. 创建数据库记录，状态为 pending
//  5. 启动异步创建过程
//  6. 释放锁
//  7. 返回实例 UUID
func (s *V2RayService) CreateInstance(ctx context.Context, region string) (string, error) {
	// 获取数据库表锁，确保串行写入
	if err := s.repo.LockTable(ctx); err != nil {
		return "", fmt.Errorf("failed to lock table: %v", err)
	}
	defer func() {
		if err := s.repo.UnlockTable(ctx); err != nil {
			logging.Error(ctx, "Failed to unlock table: %v", err)
		}
	}()

	// 检查指定region是否已有活跃实例
	hasActive, err := s.repo.CheckRegionHasActiveInstance(ctx, region)
	if err != nil {
		return "", fmt.Errorf("failed to check region for active instances: %v", err)
	}
	if hasActive {
		// 获取已存在的活跃实例
		existingInstance, err := s.repo.GetRegionActiveInstance(ctx, region)
		if err != nil {
			return "", fmt.Errorf("failed to get existing active instance: %v", err)
		}
		logging.Info(ctx, "Region %s already has active instance %d, returning existing instance", region, existingInstance.ID)
		return existingInstance.UUID, nil
	}

	// Generate UUID
	instanceUUID := uuid.New().String()

	// Create instance record with pending status
	instance := &models.V2RayInstance{
		UUID:      instanceUUID,
		EC2Region: region,
		Status:    models.StatusPending,
		IsDeleted: false,
	}

	if err := s.repo.Create(ctx, instance); err != nil {
		return "", fmt.Errorf("failed to create instance record: %v", err)
	}

	// 再次检查，确保在创建记录期间没有其他请求创建同一region的实例
	// 由于有表锁，理论上不应该发生，但作为双重保险
	allInstances, err := s.repo.List(ctx)
	if err != nil {
		return "", fmt.Errorf("failed to verify instances: %v", err)
	}
	activeCount := 0
	for _, inst := range allInstances {
		if inst.EC2Region == region && !inst.IsDeleted &&
			(inst.Status == models.StatusPending || inst.Status == models.StatusCreating || inst.Status == models.StatusRunning) {
			activeCount++
			if activeCount > 1 {
				// 发现重复，删除刚创建的记录
				logging.Warn(ctx, "Duplicate instance detected for region %s, removing newly created instance %s", region, instanceUUID)
				s.repo.Delete(ctx, instanceUUID)
				return "", fmt.Errorf("region %s already has an active instance", region)
			}
		}
	}

	// Start asynchronous creation process
	s.wg.Add(1)
	go s.createInstanceAsync(context.TODO(), instance.ID, region, instanceUUID)

	return instanceUUID, nil
}

// buildAwsUserData 构建 AWS EC2 实例的用户数据
// 参数:
//   - region: AWS 区域
//
// 返回值:
//   - string: 构建好的用户数据字符串
//
// 功能:
//  1. 定义用户数据模板，包含 V2Ray 安装、配置和启动脚本
//  2. 定义检查脚本，用于检测 V2Ray 活动状态并在不活动时终止实例
//  3. 将检查脚本编码为 base64 并替换到模板中
//  4. 替换模板中的 UUID 和端口占位符
//  5. 返回完整的用户数据字符串
func (s *V2RayService) buildAwsUserData(region, uuid string) string {
	userDataTemplate := `#!/bin/bash
# 下载v2ray安装脚本
bash <(curl -L https://github.com/v2fly/fhs-install-v2ray/raw/master/install-release.sh)
# 创建v2ray配置目录
mkdir -p /usr/local/etc/v2ray
# 生成v2ray配置文件
cat > /usr/local/etc/v2ray/config.json << EOF
{
    "log": {
        "access": "/var/log/v2ray/access.log",
        "error": "/var/log/v2ray/error.log",
        "loglevel": "info"
    },
    "inbounds": [
        {
            "port": {{Port}},
            "protocol": "vmess",
            "settings": {
                "clients": [
                    {
                        "id": "{{UUID}}",
                        "alterId": 0
                    }
                ]
            }
        }
    ],
    "outbounds": [
        {
            "protocol": "freedom",
            "settings": {}
        }
    ]
}
EOF
# 启动v2ray服务
systemctl start v2ray
systemctl enable v2ray
# 创建检查脚本，使用token方式访问实例元数据
echo {{CheckActivityScript}}|/usr/bin/base64 -d >/usr/local/bin/check_v2ray_activity.sh
# 赋予脚本执行权限
chmod +x /usr/local/bin/check_v2ray_activity.sh
# 添加到crontab，每分钟执行一次
zypper --non-interactive install cron
chcon -R -usystem_u -robject_r -tsystem_cron_spool_t /etc/crontab
systemctl enable cron
systemctl start cron
sleep 2
(crontab -l 2>/dev/null; echo "* * * * * bash /usr/local/bin/check_v2ray_activity.sh") | crontab -
chcon -R -usystem_u -robject_r -tsystem_cron_spool_t /var/spool/cron/tabs/root
systemctl restart cron`

	checkActiveScript := `#!/bin/bash
# 获取当前分钟
time=$(date +%M)

# 检查是否在每个小时的最后10分钟（50-59分钟）
if [[ "$time" -ge 50 ]]; then
	# 获取日志文件修改时间
	log_file="/var/log/v2ray/access.log"
	if [[ -f "$log_file" ]]; then
		# 计算日志文件的修改时间（秒）
		log_mtime=$(stat -c %Y "$log_file")
		# 当前时间（秒）
		current_time=$(date +%s)
		# 计算时间差（秒）
		diff=$((current_time - log_mtime))
		# 转换为分钟
		diff_minutes=$((diff / 60))

		# 检查是否超过30分钟没有修改
		if [[ "$diff_minutes" -ge 30 ]]; then
			# 1. 获取AWS元数据token
			TOKEN=$(curl -X PUT "http://169.254.169.254/latest/api/token" -H "X-aws-ec2-metadata-token-ttl-seconds: 21600" 2>/dev/null || echo "")

			# 2. 使用token直接获取实例ID和region
			if [[ -n "$TOKEN" ]]; then
				INSTANCE_ID=$(curl -H "X-aws-ec2-metadata-token: $TOKEN" http://169.254.169.254/latest/meta-data/instance-id 2>/dev/null || echo "")
				REGION=$(curl -H "X-aws-ec2-metadata-token: $TOKEN" http://169.254.169.254/latest/meta-data/placement/region 2>/dev/null || echo "")
			else
				# 兼容旧版本，尝试不使用token获取
				INSTANCE_ID=$(curl http://169.254.169.254/latest/meta-data/instance-id 2>/dev/null || echo "")
				REGION=$(curl http://169.254.169.254/latest/meta-data/placement/region 2>/dev/null || echo "")
			fi

			# 3. 终止实例
			if [[ -n "$INSTANCE_ID" && -n "$REGION" ]]; then
		rm -rf /etc/ssl/ca-bundle.pem
		cp /var/lib/ca-certificates/ca-bundle.pem /etc/ssl/
				aws ec2 terminate-instances --instance-ids "$INSTANCE_ID" --region "$REGION"
			fi
		fi
	fi
fi`

	var res = userDataTemplate
	res = strings.ReplaceAll(res, "{{CheckActivityScript}}", base64.StdEncoding.EncodeToString([]byte(checkActiveScript)))
	res = strings.ReplaceAll(res, "{{UUID}}", fmt.Sprintf("%s", uuid))
	res = strings.ReplaceAll(res, "{{Port}}", fmt.Sprintf("%d", config.AppConfig.V2Ray.Port))
	return res
}

// createInstanceAsync 异步创建 V2Ray 实例
// 参数:
//   - ctx: 上下文，用于传递请求范围的值
//   - id: 实例 ID
//   - region: AWS 区域
//   - instanceUUID: 实例 UUID
//
// 功能:
//  1. 更新上下文，添加实例 ID 用于日志记录
//  2. 更新实例状态为 creating
//  3. 创建 EC2 实例，使用构建好的用户数据
//  4. 更新数据库中的 EC2 实例 ID
//  5. 等待 EC2 实例变为运行状态
//  6. 获取实例的公网 IP 地址
//  7. 如果初始化了本地 V2Ray 管理器，将实例添加到本地配置
//  8. 更新实例状态为 running，并设置公网 IP
//  9. 记录实例创建成功的日志
func (s *V2RayService) createInstanceAsync(ctx context.Context, id int, region, instanceUUID string) {
	defer s.wg.Done()

	// Add instance ID to context for logging
	ctx = logging.WithInstanceID(ctx, instanceUUID)

	logging.Info(ctx, "Starting async creation process for instance %s in region %s", instanceUUID, region)

	// Update status to creating
	if err := s.repo.UpdateStatus(ctx, instanceUUID, models.StatusCreating); err != nil {
		logging.Error(ctx, "Failed to update status to creating: %v", err)
		return
	}

	// Create EC2 instance
	ec2ID, err := s.ec2Client.CreateInstance(ctx, region, s.buildAwsUserData(region, instanceUUID), instanceUUID)
	if err != nil {
		logging.Error(ctx, "Failed to create EC2 instance: %v", err)
		s.repo.UpdateStatus(ctx, instanceUUID, models.StatusError)
		return
	}

	// Update EC2 ID in database
	instance, err := s.repo.GetByUUID(ctx, instanceUUID)
	if err != nil {
		logging.Error(ctx, "Failed to get instance: %v", err)
		s.repo.UpdateStatus(ctx, instanceUUID, models.StatusError)
		return
	}
	instance.EC2ID = ec2ID
	if err := s.repo.Update(ctx, instance); err != nil {
		logging.Error(ctx, "Failed to update instance %s: %v", instanceUUID, err)
		s.repo.UpdateStatus(ctx, instanceUUID, models.StatusError)
		return
	}

	// Wait for instance to be running
	if err := s.ec2Client.WaitForInstanceRunning(ctx, region, ec2ID); err != nil {
		logging.Error(ctx, "Failed to wait for instance %s to be running: %v", instanceUUID, err)
		s.repo.UpdateStatus(ctx, instanceUUID, models.StatusError)
		return
	}

	// Get public IP
	publicIP, err := s.ec2Client.GetInstancePublicIP(ctx, region, ec2ID)
	if err != nil {
		logging.Error(ctx, "Failed to get public IP for instance %s: %v", instanceUUID, err)
		s.repo.UpdateStatus(ctx, instanceUUID, models.StatusError)
		return
	}

	// Add to local V2Ray config if manager is initialized
	if s.localV2RayManager != nil {
		instanceTag := fmt.Sprintf("out_aws_%s", strings.ReplaceAll(region, "-", "_"))
		if err := s.localV2RayManager.AddInstance(ctx, instanceTag, publicIP, config.AppConfig.V2Ray.Port, instanceUUID); err != nil {
			logging.Error(ctx, "Failed to add instance %s to local V2Ray config: %v", instanceUUID, err)
			// Continue even if local config update fails
		} else {
			logging.Info(ctx, "Added instance %s to local V2Ray config", instanceTag)
		}
	}

	if err := s.repo.UpdateStatusAndIP(ctx, instanceUUID, models.StatusRunning, publicIP); err != nil {
		logging.Error(ctx, "Failed to update status to running: %v", err)
		return
	}

	logging.Info(ctx, "Instance %s created successfully with public IP: %s", instanceUUID, publicIP)
}

// ListInstances 获取所有未删除的 V2Ray 实例列表
// 参数:
//   - ctx: 上下文，用于传递请求范围的值
//
// 返回值:
//   - []*models.V2RayInstance: V2Ray 实例列表
//   - error: 错误信息，如果获取失败
//
// 功能:
//  1. 调用仓库层的 List 方法获取实例列表
//  2. 返回实例列表和可能的错误
func (s *V2RayService) ListInstances(ctx context.Context) ([]*models.V2RayInstance, error) {
	instances, err := s.repo.List(ctx)
	if err != nil {
		return nil, err
	}

	for _, instance := range instances {
		if regionConfig, ok := config.AppConfig.AWS.Regions[instance.EC2Region]; ok {
			instance.EC2RegionName = regionConfig.Name
		}
	}

	return instances, nil
}

// GetInstance 根据 ID 获取 V2Ray 实例详情
// 参数:
//   - ctx: 上下文，用于传递请求范围的值
//   - id: 实例 ID
//
// 返回值:
//   - *models.V2RayInstance: 找到的 V2Ray 实例
//   - error: 错误信息，如果获取失败
//
// 功能:
//  1. 调用仓库层的 GetByID 方法获取实例详情
//  2. 返回实例详情和可能的错误
func (s *V2RayService) GetInstance(ctx context.Context, uuid string) (*models.V2RayInstance, error) {
	instance, err := s.repo.GetByUUID(ctx, uuid)
	if err != nil {
		return nil, err
	}

	if regionConfig, ok := config.AppConfig.AWS.Regions[instance.EC2Region]; ok {
		instance.EC2RegionName = regionConfig.Name
	}

	return instance, nil
}

// DeleteInstance 删除 V2Ray 实例
// 参数:
//   - ctx: 上下文，用于传递请求范围的值
//   - id: 实例 ID
//
// 返回值:
//   - error: 错误信息，如果删除失败
//
// 功能:
//  1. 根据 ID 获取实例详情
//  2. 更新实例状态为 deleting
//  3. 启动异步删除过程
//  4. 返回可能的错误
func (s *V2RayService) DeleteInstance(ctx context.Context, uuid string) error {
	// Get instance
	instance, err := s.repo.GetByUUID(ctx, uuid)
	if err != nil {
		return fmt.Errorf("instance not found: %v", err)
	}

	// Update status to deleted
	if err := s.repo.UpdateStatus(ctx, uuid, models.StatusDeleting); err != nil {
		return fmt.Errorf("failed to update status: %v", err)
	}

	// Start asynchronous deletion process
	s.wg.Add(1)
	go s.deleteInstanceAsync(ctx, uuid, instance.EC2ID, instance.EC2Region)

	return nil
}

// deleteInstanceAsync 异步删除 V2Ray 实例
// 参数:
//   - ctx: 上下文，用于传递请求范围的值
//   - id: 实例 ID
//   - ec2ID: EC2 实例 ID
//   - region: AWS 区域
//
// 功能:
//  1. 更新上下文，添加实例 ID 用于日志记录
//  2. 更新实例状态为 deleting
//  3. 如果初始化了本地 V2Ray 管理器，从本地配置中移除实例
//  4. 终止 EC2 实例
//  5. 等待 EC2 实例变为终止状态
//  6. 标记数据库中的实例为已删除
//  7. 记录实例删除成功的日志
func (s *V2RayService) deleteInstanceAsync(ctx context.Context, uuid string, ec2ID, region string) {
	defer s.wg.Done()

	// Add instance ID to context for logging
	ctx = logging.WithInstanceID(ctx, uuid)

	logging.Info(ctx, "Starting async deletion process for instance %s (EC2: %s)", uuid, ec2ID)

	// Update status to deleted
	if err := s.repo.UpdateStatus(ctx, uuid, models.StatusDeleting); err != nil {
		logging.Error(ctx, "Failed to update status to deleting: %v", err)
		return
	}

	// Terminate EC2 instance
	if err := s.ec2Client.TerminateInstance(ctx, region, ec2ID); err != nil {
		logging.Error(ctx, "Failed to terminate EC2 instance: %v", err)
		s.repo.UpdateStatus(ctx, uuid, models.StatusError)
		return
	}

	// Wait for instance to be terminated
	if err := s.ec2Client.WaitForInstanceTerminated(ctx, region, ec2ID); err != nil {
		logging.Error(ctx, "Failed to wait for instance terminated: %v", err)
		s.repo.UpdateStatus(ctx, uuid, models.StatusError)
		return
	}

	// Update status to deleted
	if err := s.repo.Delete(ctx, uuid); err != nil {
		logging.Error(ctx, "Failed to update status to deleted: %v", err)
		return
	}

	logging.Info(ctx, "Instance %s deleted successfully", uuid)
}

// ListRegions 列出所有支持的 AWS 区域
// 参数:
//   - ctx: 上下文，用于传递请求范围的值
//
// 返回值:
//   - []*models.Region: 区域列表
//
// 功能:
//  1. 从配置文件中获取所有配置的区域
//  2. 返回区域代码和名称的列表
func (s *V2RayService) ListRegions(ctx context.Context) []*models.Region {
	var regions []*models.Region

	for regionCode, regionConfig := range config.AppConfig.AWS.Regions {
		regions = append(regions, &models.Region{
			Region: regionCode,
			Name:   regionConfig.Name,
		})
	}

	return regions
}

// Wait 等待所有异步操作完成
// 功能:
//  1. 阻塞直到所有通过 WaitGroup 跟踪的异步操作完成
//  2. 用于确保在程序退出前所有异步任务都已完成
func (s *V2RayService) Wait() {
	s.wg.Wait()
}
