package repository

import (
	"context"
	"fmt"

	"github.com/jmoiron/sqlx"
	"github.com/yuhai94/anywhere_backend/internal/logging"
	"github.com/yuhai94/anywhere_backend/internal/models"
)

type Repository struct {
	db *sqlx.DB
}

// New 创建一个新的 Repository 实例
// 参数:
//   - db: sqlx.DB 实例，用于数据库操作
//
// 返回值:
//   - *Repository: 新创建的 Repository 实例
//
// 功能:
//  1. 初始化 Repository 结构体
//  2. 设置数据库连接
func New(db *sqlx.DB) *Repository {
	return &Repository{db: db}
}

// Create 创建 V2Ray 实例记录
// 参数:
//   - ctx: 上下文，用于传递请求范围的值
//   - instance: 要创建的 V2Ray 实例
//
// 返回值:
//   - error: 错误信息，如果创建失败
//
// 功能:
//  1. 执行插入操作，将实例信息插入到数据库
//  2. 获取插入后的自增 ID
//  3. 将 ID 设置到实例对象中
//  4. 记录创建成功的日志
func (r *Repository) Create(ctx context.Context, instance *models.V2RayInstance) error {
	query := `
		INSERT INTO v2ray_instances (uuid, ec2_id, ec2_region, status, is_deleted)
		VALUES (?, ?, ?, ?, ?)
	`
	result, err := r.db.ExecContext(ctx, query, instance.UUID, instance.EC2ID, instance.EC2Region, instance.Status, instance.IsDeleted)
	if err != nil {
		logging.Error(ctx, "Failed to create instance: %v", err)
		return err
	}

	id, err := result.LastInsertId()
	if err != nil {
		logging.Error(ctx, "Failed to get last insert id: %v", err)
		return err
	}

	instance.ID = int(id)
	logging.Info(ctx, "Created instance with ID: %d", instance.ID)
	return nil
}

// GetByUUID 根据 UUID 获取 V2Ray 实例
// 参数:
//   - ctx: 上下文，用于传递请求范围的值
//   - uuid: 实例 UUID
//
// 返回值:
//   - *models.V2RayInstance: 找到的 V2Ray 实例
//   - error: 错误信息，如果获取失败
//
// 功能:
//  1. 执行查询操作，根据 UUID 查找未删除的实例
//  2. 将查询结果扫描到实例对象中
//  3. 返回实例对象
func (r *Repository) GetByUUID(ctx context.Context, uuid string) (*models.V2RayInstance, error) {
	var instance models.V2RayInstance
	query := `SELECT * FROM v2ray_instances WHERE uuid = ? AND is_deleted = false`
	err := r.db.GetContext(ctx, &instance, query, uuid)
	if err != nil {
		logging.Error(ctx, "Failed to get instance by UUID %s: %v", uuid, err)
		return nil, err
	}
	return &instance, nil
}

// List 获取所有未删除的 V2Ray 实例列表
// 参数:
//   - ctx: 上下文，用于传递请求范围的值
//
// 返回值:
//   - []*models.V2RayInstance: V2Ray 实例列表
//   - error: 错误信息，如果获取失败
//
// 功能:
//  1. 执行查询操作，获取所有未删除的实例
//  2. 按创建时间倒序排序
//  3. 将查询结果扫描到实例列表中
//  4. 返回实例列表
func (r *Repository) List(ctx context.Context) ([]*models.V2RayInstance, error) {
	var instances []*models.V2RayInstance
	query := `SELECT * FROM v2ray_instances WHERE is_deleted = false ORDER BY created_at DESC`
	err := r.db.SelectContext(ctx, &instances, query)
	if err != nil {
		logging.Error(ctx, "Failed to list instances: %v", err)
		return nil, err
	}
	return instances, nil
}

// Update 更新 V2Ray 实例记录
// 参数:
//   - ctx: 上下文，用于传递请求范围的值
//   - instance: 要更新的 V2Ray 实例
//
// 返回值:
//   - error: 错误信息，如果更新失败
//
// 功能:
//  1. 执行更新操作，更新实例的所有字段
//  2. 记录更新操作的结果
//  3. 返回更新操作的错误信息
func (r *Repository) Update(ctx context.Context, instance *models.V2RayInstance) error {
	query := `
		UPDATE v2ray_instances
		SET ec2_id = ?, ec2_region = ?, ec2_public_ip = ?, status = ?, 
		    direct_link = ?, relay_link = ?, is_deleted = ?
		WHERE uuid = ?
	`
	_, err := r.db.ExecContext(ctx, query,
		instance.EC2ID, instance.EC2Region, instance.EC2PublicIP,
		instance.Status, instance.DirectLink, instance.RelayLink,
		instance.IsDeleted, instance.UUID,
	)
	if err != nil {
		logging.Error(ctx, "Failed to update instance %s: %v", instance.UUID, err)
		return err
	}
	logging.Info(ctx, "Updated instance %s with status: %s", instance.UUID, instance.Status)
	return nil
}

// UpdateLinks 更新 V2Ray 实例的链接字段
// 参数:
//   - ctx: 上下文，用于传递请求范围的值
//   - uuid: 实例 UUID
//   - directLink: 直连链接
//   - relayLink: 中转链接
//
// 返回值:
//   - error: 错误信息，如果更新失败
func (r *Repository) UpdateLinks(ctx context.Context, uuid, directLink, relayLink string) error {
	query := `UPDATE v2ray_instances SET direct_link = ?, relay_link = ? WHERE uuid = ?`
	_, err := r.db.ExecContext(ctx, query, directLink, relayLink, uuid)
	if err != nil {
		logging.Error(ctx, "Failed to update links for instance %s: %v", uuid, err)
		return err
	}
	logging.Info(ctx, "Updated links for instance %s", uuid)
	return nil
}

// UpdateStatus 更新 V2Ray 实例的状态
// 参数:
//   - ctx: 上下文，用于传递请求范围的值
//   - id: 实例 ID
//   - status: 新的状态
//
// 返回值:
//   - error: 错误信息，如果更新失败
//
// 功能:
//  1. 执行更新操作，更新实例的状态
//  2. 记录状态更新的结果
//  3. 返回更新操作的错误信息
func (r *Repository) UpdateStatus(ctx context.Context, uuid string, status string) error {
	query := `UPDATE v2ray_instances SET status = ? WHERE uuid = ?`
	_, err := r.db.ExecContext(ctx, query, status, uuid)
	if err != nil {
		logging.Error(ctx, "Failed to update status for instance %s: %v", uuid, err)
		return err
	}
	logging.Info(ctx, "Updated status for instance %s to: %s", uuid, status)
	return nil
}

// UpdateStatusAndIP 更新 V2Ray 实例的状态和公网 IP
// 参数:
//   - ctx: 上下文，用于传递请求范围的值
//   - uuid: 实例 UUID
//   - status: 新的状态
//   - publicIP: 新的公网 IP
//
// 返回值:
//   - error: 错误信息，如果更新失败
//
// 功能:
//  1. 执行更新操作，同时更新实例的状态和公网 IP
//  2. 记录状态和 IP 更新的结果
//  3. 返回更新操作的错误信息
func (r *Repository) UpdateStatusAndIP(ctx context.Context, uuid string, status string, publicIP string) error {
	query := `UPDATE v2ray_instances SET status = ?, ec2_public_ip = ? WHERE uuid = ?`
	_, err := r.db.ExecContext(ctx, query, status, publicIP, uuid)
	if err != nil {
		logging.Error(ctx, "Failed to update status and IP for instance %s: %v", uuid, err)
		return err
	}
	logging.Info(ctx, "Updated status for instance %s to: %s, IP: %s", uuid, status, publicIP)
	return nil
}

// Delete 标记 V2Ray 实例为已删除
// 参数:
//   - ctx: 上下文，用于传递请求范围的值
//   - uuid: 实例 UUID
//
// 返回值:
//   - error: 错误信息，如果删除失败
//
// 功能:
//  1. 执行更新操作，将实例状态设置为已删除
//  2. 将 is_deleted 字段设置为 true
//  3. 记录删除操作的结果
//  4. 返回删除操作的错误信息
func (r *Repository) Delete(ctx context.Context, uuid string) error {
	query := `UPDATE v2ray_instances SET status = ?, is_deleted = true WHERE uuid = ?`
	_, err := r.db.ExecContext(ctx, query, models.StatusDeleted, uuid)
	if err != nil {
		logging.Error(ctx, "Failed to delete instance %s: %v", uuid, err)
		return err
	}
	logging.Info(ctx, "Deleted instance %s", uuid)
	return nil
}

// CheckRegionHasActiveInstance 检查指定region是否存在活跃实例
// 参数:
//   - ctx: 上下文，用于传递请求范围的值
//   - region: AWS区域
//
// 返回值:
//   - bool: 如果存在活跃实例返回true，否则返回false
//   - error: 错误信息，如果检查失败
//
// 功能:
//  1. 检查指定region是否存在pending、creating或running状态的实例
//  2. 返回检查结果
func (r *Repository) CheckRegionHasActiveInstance(ctx context.Context, region string) (bool, error) {
	query := `
		SELECT COUNT(*) FROM v2ray_instances 
		WHERE ec2_region = ? AND is_deleted = false 
		AND status IN (?, ?, ?)
	`
	var count int
	err := r.db.GetContext(ctx, &count, query, region, models.StatusPending, models.StatusCreating, models.StatusRunning)
	if err != nil {
		logging.Error(ctx, "Failed to check region %s for active instances: %v", region, err)
		return false, err
	}
	return count > 0, nil
}

// GetRegionActiveInstance 获取指定region的活跃实例
// 参数:
//   - ctx: 上下文，用于传递请求范围的值
//   - region: AWS区域
//
// 返回值:
//   - *models.V2RayInstance: 活跃实例对象
//   - error: 错误信息，如果获取失败
//
// 功能:
//  1. 获取指定region的pending、creating或running状态的实例
//  2. 返回获取到的实例
func (r *Repository) GetRegionActiveInstance(ctx context.Context, region string) (*models.V2RayInstance, error) {
	query := `
		SELECT * FROM v2ray_instances 
		WHERE ec2_region = ? AND is_deleted = false 
		AND status IN (?, ?, ?) 
		LIMIT 1
	`
	var instance models.V2RayInstance
	err := r.db.GetContext(ctx, &instance, query, region, models.StatusPending, models.StatusCreating, models.StatusRunning)
	if err != nil {
		logging.Error(ctx, "Failed to get active instance for region %s: %v", region, err)
		return nil, err
	}
	return &instance, nil
}

// LockTable 锁定表，用于实现串行写入
// 参数:
//   - ctx: 上下文，用于传递请求范围的值
//
// 返回值:
//   - error: 错误信息，如果锁定失败
//
// 功能:
//  1. 执行表锁定操作
func (r *Repository) LockTable(ctx context.Context) error {
	query := `LOCK TABLES v2ray_instances WRITE`
	_, err := r.db.ExecContext(ctx, query)
	if err != nil {
		logging.Error(ctx, "Failed to lock table: %v", err)
		return err
	}
	return nil
}

// UnlockTable 解锁表
// 参数:
//   - ctx: 上下文，用于传递请求范围的值
//
// 返回值:
//   - error: 错误信息，如果解锁失败
//
// 功能:
//  1. 执行表解锁操作
func (r *Repository) UnlockTable(ctx context.Context) error {
	query := `UNLOCK TABLES`
	_, err := r.db.ExecContext(ctx, query)
	if err != nil {
		logging.Error(ctx, "Failed to unlock table: %v", err)
		return err
	}
	return nil
}

// InitSchema 初始化数据库表结构
// 参数:
//   - ctx: 上下文，用于传递请求范围的值
//
// 返回值:
//   - error: 错误信息，如果初始化失败
//
// 功能:
//  1. 执行 SQL 语句创建 v2ray_instances 表
//  2. 表包含 id、uuid、ec2_id、ec2_region、ec2_public_ip、status、created_at、updated_at、is_deleted 字段
//  3. 添加适当的索引和注释
//  4. 记录初始化结果
func (r *Repository) InitSchema(ctx context.Context) error {
	schema := `
		CREATE TABLE IF NOT EXISTS v2ray_instances (
			id INT NOT NULL AUTO_INCREMENT COMMENT '实例 ID (自增)',
			uuid VARCHAR(36) NOT NULL COMMENT 'V2Ray 客户端 UUID',
			ec2_id VARCHAR(255) NOT NULL COMMENT 'AWS EC2 实例 ID',
			ec2_region VARCHAR(100) NOT NULL COMMENT 'AWS 区域',
			ec2_public_ip VARCHAR(50) NOT NULL DEFAULT '' COMMENT '公网 IP 地址',
			status VARCHAR(50) NOT NULL COMMENT '实例状态（pending, creating, running, deleting, deleted, error）',
			direct_link TEXT NOT NULL DEFAULT '' COMMENT '直连链接',
			relay_link TEXT NOT NULL DEFAULT '' COMMENT '中转链接',
			created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP COMMENT '创建时间',
			updated_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP COMMENT '最后更新时间',
			is_deleted BOOLEAN NOT NULL DEFAULT FALSE COMMENT '删除标志',
			PRIMARY KEY (id),
			INDEX idx_status (status),
			INDEX idx_is_deleted (is_deleted)
		) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci COMMENT='V2Ray 实例表';
	`
	_, err := r.db.ExecContext(ctx, schema)
	if err != nil {
		logging.Error(ctx, "Failed to create schema: %v", err)
		return fmt.Errorf("failed to create schema: %v", err)
	}
	logging.Info(ctx, "Database schema initialized")
	return nil
}
