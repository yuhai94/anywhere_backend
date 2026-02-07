package aws

import (
	"context"
	"encoding/base64"
	"fmt"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	awsconfig "github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/credentials"
	"github.com/aws/aws-sdk-go-v2/service/ec2"
	ec2types "github.com/aws/aws-sdk-go-v2/service/ec2/types"
	appconfig "github.com/yuhai94/anywhere_backend/internal/config"
	"github.com/yuhai94/anywhere_backend/internal/logging"
	"github.com/yuhai94/anywhere_backend/internal/models"
)

type EC2Client struct {
	clients map[string]*ec2.Client
}

// NewEC2Client 创建一个新的 EC2Client 实例
// 返回值:
//   - *EC2Client: 新创建的 EC2Client 实例
//   - error: 错误信息，如果创建失败
//
// 功能:
//  1. 为配置文件中定义的每个 AWS 区域创建 EC2 客户端
//  2. 返回包含所有区域客户端的 EC2Client 实例
func NewEC2Client() (*EC2Client, error) {
	clients := make(map[string]*ec2.Client)

	for region := range appconfig.AppConfig.AWS.Regions {
		cfg, err := awsconfig.LoadDefaultConfig(context.Background(),
			awsconfig.WithRegion(region),
			awsconfig.WithCredentialsProvider(credentials.StaticCredentialsProvider{
				Value: aws.Credentials{
					AccessKeyID: appconfig.AppConfig.AWS.AccessKey, SecretAccessKey: appconfig.AppConfig.AWS.SecretKey,
				},
			}))

		if err != nil {
			return nil, fmt.Errorf("failed to load config for region %s: %v", region, err)
		}

		clients[region] = ec2.NewFromConfig(cfg)
	}

	return &EC2Client{clients: clients}, nil
}

// CreateInstance 创建 EC2 实例
// 参数:
//   - ctx: 上下文，用于传递请求范围的值和取消信号
//   - region: AWS 区域
//   - userData: 实例启动时执行的用户数据
//
// 返回值:
//   - string: 创建的 EC2 实例 ID
//   - error: 错误信息，如果创建失败
//
// 功能:
//  1. 获取指定区域的 EC2 客户端
//  2. 获取区域配置信息
//  3. 使用启动模板创建 EC2 实例
//  4. 返回创建的实例 ID
func (e *EC2Client) CreateInstance(ctx context.Context, region string, userData string, uuid string) (string, error) {
	client, ok := e.clients[region]
	if !ok {
		return "", fmt.Errorf("no client configured for region %s", region)
	}

	regionConfig, err := appconfig.GetRegionConfig(region)
	if err != nil {
		return "", err
	}

	logging.Info(ctx, "Creating EC2 instance in region %s with template %s, userData: %s", region, regionConfig.TemplateID, userData)

	input := &ec2.RunInstancesInput{
		LaunchTemplate: &ec2types.LaunchTemplateSpecification{
			LaunchTemplateId: aws.String(regionConfig.TemplateID),
		},
		MinCount: aws.Int32(1),
		MaxCount: aws.Int32(1),
		UserData: aws.String(base64.StdEncoding.EncodeToString([]byte(userData))),
		TagSpecifications: []ec2types.TagSpecification{
			{
				ResourceType: ec2types.ResourceTypeInstance,
				Tags: []ec2types.Tag{
					{
						Key:   aws.String("UUID"),
						Value: aws.String(uuid),
					},
				},
			},
		},
	}

	resp, err := client.RunInstances(ctx, input)
	if err != nil {
		logging.EC2Log(ctx, "run_instances", region, "", map[string]interface{}{
			"launch_template_id": regionConfig.TemplateID,
		}, err)
		return "", fmt.Errorf("failed to run instances: %v", err)
	}

	if len(resp.Instances) == 0 {
		return "", fmt.Errorf("no instances created")
	}

	instanceID := *resp.Instances[0].InstanceId
	logging.EC2Log(ctx, "run_instances", region, instanceID, map[string]interface{}{
		"launch_template_id": regionConfig.TemplateID,
		"user_data":          userData,
	}, nil)

	return instanceID, nil
}

// WaitForInstanceRunning 等待 EC2 实例变为运行状态
// 参数:
//   - ctx: 上下文，用于传递请求范围的值和取消信号
//   - region: AWS 区域
//   - instanceID: EC2 实例 ID
//
// 返回值:
//   - error: 错误信息，如果等待失败或超时
//
// 功能:
//  1. 获取指定区域的 EC2 客户端
//  2. 循环检查实例状态
//  3. 当实例变为运行状态时返回成功
//  4. 当实例变为终止或关闭状态时返回错误
//  5. 当等待超过 5 分钟时返回超时错误
func (e *EC2Client) WaitForInstanceRunning(ctx context.Context, region string, instanceID string) error {
	client, ok := e.clients[region]
	if !ok {
		return fmt.Errorf("no client configured for region %s", region)
	}

	logging.Info(ctx, "Waiting for instance %s in region %s to be running", instanceID, region)

	start := time.Now()
	for {
		input := &ec2.DescribeInstancesInput{
			InstanceIds: []string{instanceID},
		}

		resp, err := client.DescribeInstances(ctx, input)
		if err != nil {
			logging.EC2Log(ctx, "describe_instances", region, instanceID, nil, err)
		} else {
			logging.Info(ctx, "describe_instances return %+v", resp)

			if len(resp.Reservations) > 0 && len(resp.Reservations[0].Instances) > 0 {
				instance := resp.Reservations[0].Instances[0]
				status := instance.State.Name
				logging.Info(ctx, "Instance %s status: %s", instanceID, status)

				if status == ec2types.InstanceStateNameRunning {
					logging.EC2Log(ctx, "wait_running", region, instanceID, map[string]interface{}{
						"elapsed_time": time.Since(start).String(),
					}, nil)
					return nil
				}

				if status == ec2types.InstanceStateNameTerminated || status == ec2types.InstanceStateNameShuttingDown {
					return fmt.Errorf("instance %s is %s", instanceID, status)
				}
			}
		}

		time.Sleep(5 * time.Second)
		if time.Since(start) > time.Duration(appconfig.AppConfig.Scheduler.InstanceWaitTimeout)*time.Second {
			return fmt.Errorf("timeout waiting for instance %s to be running", instanceID)
		}
	}
}

// GetInstancePublicIP 获取 EC2 实例的公网 IP 地址
// 参数:
//   - ctx: 上下文，用于传递请求范围的值和取消信号
//   - region: AWS 区域
//   - instanceID: EC2 实例 ID
//
// 返回值:
//   - string: 实例的公网 IP 地址
//   - error: 错误信息，如果获取失败
//
// 功能:
//  1. 获取指定区域的 EC2 客户端
//  2. 描述实例详情
//  3. 检查实例是否存在
//  4. 检查实例是否有公网 IP 地址
//  5. 返回实例的公网 IP 地址
func (e *EC2Client) GetInstancePublicIP(ctx context.Context, region string, instanceID string) (string, error) {
	client, ok := e.clients[region]
	if !ok {
		return "", fmt.Errorf("no client configured for region %s", region)
	}

	input := &ec2.DescribeInstancesInput{
		InstanceIds: []string{instanceID},
	}

	resp, err := client.DescribeInstances(ctx, input)
	if err != nil {
		logging.EC2Log(ctx, "describe_instances", region, instanceID, nil, err)
		return "", fmt.Errorf("failed to describe instances: %v", err)
	}
	logging.Info(ctx, "describe_instances return %+v", resp)

	if len(resp.Reservations) == 0 || len(resp.Reservations[0].Instances) == 0 {
		return "", fmt.Errorf("instance %s not found", instanceID)
	}

	instance := resp.Reservations[0].Instances[0]
	if instance.PublicIpAddress == nil {
		return "", fmt.Errorf("instance %s has no public IP", instanceID)
	}

	publicIP := *instance.PublicIpAddress
	logging.EC2Log(ctx, "get_public_ip", region, instanceID, map[string]interface{}{
		"public_ip": publicIP,
	}, nil)

	return publicIP, nil
}

// TerminateInstance 终止 EC2 实例
// 参数:
//   - ctx: 上下文，用于传递请求范围的值和取消信号
//   - region: AWS 区域
//   - instanceID: EC2 实例 ID
//
// 返回值:
//   - error: 错误信息，如果终止失败
//
// 功能:
//  1. 获取指定区域的 EC2 客户端
//  2. 发送终止实例请求
//  3. 检查是否有实例被终止
//  4. 记录终止操作的日志
func (e *EC2Client) TerminateInstance(ctx context.Context, region string, instanceID string) error {
	client, ok := e.clients[region]
	if !ok {
		return fmt.Errorf("no client configured for region %s", region)
	}

	logging.Info(ctx, "Terminating instance %s in region %s", instanceID, region)

	input := &ec2.TerminateInstancesInput{
		InstanceIds: []string{instanceID},
	}

	resp, err := client.TerminateInstances(ctx, input)
	if err != nil {
		logging.EC2Log(ctx, "terminate_instances", region, instanceID, nil, err)
		return fmt.Errorf("failed to terminate instances: %v", err)
	}

	if len(resp.TerminatingInstances) == 0 {
		return fmt.Errorf("no instances terminated")
	}

	logging.EC2Log(ctx, "terminate_instances", region, instanceID, map[string]interface{}{
		"current_state":  string(resp.TerminatingInstances[0].CurrentState.Name),
		"previous_state": string(resp.TerminatingInstances[0].PreviousState.Name),
	}, nil)

	return nil
}

// InstanceInfo 存储实例信息
type InstanceInfo struct {
	InstanceID string
	Region     string
	PublicIP   string
	UUID       string
	Status     string
}

// ConvertInstanceStateToModelStatus 将 AWS 实例状态转换为模型状态
// 参数:
//   - state: AWS 实例状态
//
// 返回值:
//   - string: 对应的模型状态
func ConvertInstanceStateToModelStatus(state ec2types.InstanceStateName) string {
	switch state {
	case ec2types.InstanceStateNamePending:
		return models.StatusCreating
	case ec2types.InstanceStateNameRunning:
		return models.StatusRunning
	case ec2types.InstanceStateNameShuttingDown:
		return models.StatusDeleting
	case ec2types.InstanceStateNameTerminated:
		return models.StatusDeleted
	case ec2types.InstanceStateNameStopped:
		return models.StatusDeleted
	case ec2types.InstanceStateNameStopping:
		return models.StatusDeleting
	default:
		return models.StatusError
	}
}

// DescribeInstances 获取指定区域的所有 EC2 实例信息
// 参数:
//   - ctx: 上下文，用于传递请求范围的值和取消信号
//   - region: AWS 区域
//
// 返回值:
//   - []InstanceInfo: 实例信息列表
//   - error: 错误信息，如果获取失败
//
// 功能:
//  1. 获取指定区域的 EC2 客户端
//  2. 调用 DescribeInstances API 获取实例列表
//  3. 从响应中提取实例的 ID、区域、公网 IP 和 UUID 标签
//  4. 返回实例信息列表
func (e *EC2Client) DescribeInstances(ctx context.Context, region string) ([]InstanceInfo, error) {
	client, ok := e.clients[region]
	if !ok {
		return nil, fmt.Errorf("no client configured for region %s", region)
	}

	logging.Info(ctx, "Describing EC2 instances in region %s", region)

	input := &ec2.DescribeInstancesInput{}
	resp, err := client.DescribeInstances(ctx, input)
	if err != nil {
		logging.EC2Log(ctx, "describe_instances", region, "", nil, err)
		return nil, fmt.Errorf("failed to describe instances: %v", err)
	}

	var instances []InstanceInfo
	for _, reservation := range resp.Reservations {
		for _, instance := range reservation.Instances {
			// 跳过终止状态的实例
			if instance.State.Name == ec2types.InstanceStateNameTerminated {
				continue
			}

			instanceID := *instance.InstanceId
			publicIP := ""
			if instance.PublicIpAddress != nil {
				publicIP = *instance.PublicIpAddress
			}

			// 提取 UUID 标签
			uuid := ""
			for _, tag := range instance.Tags {
				if *tag.Key == "UUID" {
					uuid = *tag.Value
					break
				}
			}

			// 转换 AWS 状态为模型状态
			modelStatus := ConvertInstanceStateToModelStatus(instance.State.Name)

			instances = append(instances, InstanceInfo{
				InstanceID: instanceID,
				Region:     region,
				PublicIP:   publicIP,
				UUID:       uuid,
				Status:     modelStatus,
			})
		}
	}

	logging.Info(ctx, "Found %d EC2 instances in region %s", len(instances), region)
	return instances, nil
}

// WaitForInstanceTerminated 等待 EC2 实例变为终止状态
// 参数:
//   - ctx: 上下文，用于传递请求范围的值和取消信号
//   - region: AWS 区域
//   - instanceID: EC2 实例 ID
//
// 返回值:
//   - error: 错误信息，如果等待失败或超时
//
// 功能:
//  1. 获取指定区域的 EC2 客户端
//  2. 循环检查实例状态
//  3. 当实例不存在时返回成功
//  4. 当实例变为终止状态时返回成功
//  5. 当等待超过 5 分钟时返回超时错误
func (e *EC2Client) WaitForInstanceTerminated(ctx context.Context, region string, instanceID string) error {
	client, ok := e.clients[region]
	if !ok {
		return fmt.Errorf("no client configured for region %s", region)
	}

	logging.Info(ctx, "Waiting for instance %s in region %s to be terminated", instanceID, region)

	start := time.Now()
	for {
		input := &ec2.DescribeInstancesInput{
			InstanceIds: []string{instanceID},
		}

		resp, err := client.DescribeInstances(ctx, input)
		if err != nil {
			logging.EC2Log(ctx, "describe_instances", region, instanceID, nil, err)
			return fmt.Errorf("failed to describe instances: %v", err)
		}

		if len(resp.Reservations) == 0 || len(resp.Reservations[0].Instances) == 0 {
			logging.EC2Log(ctx, "wait_terminated", region, instanceID, map[string]interface{}{
				"elapsed_time": time.Since(start).String(),
			}, nil)
			return nil
		}

		instance := resp.Reservations[0].Instances[0]
		status := instance.State.Name
		logging.Info(ctx, "Instance %s termination status: %s", instanceID, status)

		if status == ec2types.InstanceStateNameTerminated {
			logging.EC2Log(ctx, "wait_terminated", region, instanceID, map[string]interface{}{
				"elapsed_time": time.Since(start).String(),
			}, nil)
			return nil
		}

		time.Sleep(5 * time.Second)
		if time.Since(start) > time.Duration(appconfig.AppConfig.Scheduler.InstanceWaitTimeout)*time.Second {
			return fmt.Errorf("timeout waiting for instance %s to be terminated", instanceID)
		}
	}
}
