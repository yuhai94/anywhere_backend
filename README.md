# V2Ray 后端服务

这是一个使用 Golang 开发的 V2Ray 实例管理后端服务，提供创建、列出、删除 V2Ray 实例的 API 接口。

## 功能特性

- **创建 V2Ray 实例**：异步创建 AWS EC2 实例并安装配置 V2Ray
- **列出 V2Ray 实例**：获取当前所有 V2Ray 实例的状态和信息
- **获取实例详情**：获取单个 V2Ray 实例的详细信息
- **删除 V2Ray 实例**：异步删除指定的 V2Ray 实例
- **列出支持的区域**：获取配置文件中所有支持的 AWS 区域列表
- **本地 V2Ray 管理**：自动将新创建的实例添加到本地 V2Ray 配置中作为中转节点
- **完整的日志系统**：详细记录所有操作，包括 EC2 交互
- **状态管理**：完善的实例状态管理和错误处理
- **并发安全**：使用数据库表锁确保同一 region 只创建一个实例
- **自动同步**：定期同步 AWS 实例状态到数据库

## 技术栈

- **语言**：Golang 1.25.5
- **Web 框架**：Gin
- **数据库**：MySQL
- **AWS SDK**：AWS SDK for Go v2
- **配置管理**：YAML
- **日志系统**：zap

## 项目结构

```
├── cmd/
│   └── api/
│       └── main.go        # API 服务器入口点
├── internal/
│   ├── api/
│   │   ├── handlers/      # API 处理器
│   │   └── routes/        # 路由定义
│   ├── service/           # 业务逻辑层
│   ├── repository/        # 数据访问层
│   ├── interfaces/        # 接口定义
│   ├── aws/              # AWS 集成
│   ├── config/           # 配置管理
│   ├── models/           # 数据模型
│   ├── logging/          # 日志系统
│   ├── scheduler/        # 定时任务
│   └── localv2ray/      # 本地 V2Ray 管理
├── conf/
│   └── conf.yaml        # YAML 配置文件
├── logs/                  # 日志文件目录
├── scripts/               # 实用脚本
├── go.mod                 # Go 模块
└── go.sum                 # 依赖校验和
```

## 配置说明

### 配置文件

配置文件位于 `conf/conf.yaml`，包含以下主要部分：

- **server**：服务器配置
- **database**：数据库连接配置
- **aws**：AWS 相关配置，包括凭证和区域配置
- **v2ray**：V2Ray 安装和配置模板
- **logging**：日志系统配置
- **scheduler**：定时任务配置

### AWS 配置

在 `aws` 部分，需要配置：
- `access_key`：AWS 访问密钥
- `secret_key`：AWS 秘密密钥
- `regions`：各个区域的配置，包括：
  - `template_id`：启动模板 ID
  - `name`：区域中文名称

### V2Ray 配置

在 `v2ray` 部分，需要配置：
- `local_config_path`：本地 V2Ray 配置文件路径，用于自动管理本地 V2Ray 配置
- `port`：V2Ray 服务端口（默认 11994）

### Scheduler 配置

在 `scheduler` 部分，需要配置：
- `instance_sync_interval`：AWS 实例同步间隔，单位秒（默认 60 秒）
- `instance_wait_timeout`：实例等待超时时间，单位秒（默认 300 秒）

## API 接口

### 列出支持的区域

获取配置文件中所有支持的 AWS 区域列表。

- **方法**：GET
- **路径**：`/api/v2ray/regions`
- **成功响应**（200）：
  ```json
  [
    {
      "region": "ap-east-1",
      "name": "香港"
    },
    {
      "region": "us-west-2",
      "name": "美西"
    }
  ]
  ```

### 创建 V2Ray 实例

创建一个新的 V2Ray 实例，系统会自动检查指定 region 是否已有活跃实例。

- **方法**：POST
- **路径**：`/api/v2ray/instances`
- **请求体**：
  ```json
  {
    "region": "us-east-1"
  }
  ```
- **成功响应**（200）：
  ```json
  {
    "uuid": "550e8400-e29b-41d4-a716-446655440000",
    "status": "pending"
  }
  ```
- **错误响应**（400）：
  ```json
  {
    "error": "region already has an active instance"
  }
  ```
- **错误响应**（500）：
  ```json
  {
    "error": "failed to create instance record: ..."
  }
  ```

**说明**：
- 如果指定 region 已有活跃实例（pending/creating/running 状态），将返回已有实例的 UUID
- 创建操作是异步的，返回的 UUID 可用于查询实例状态
- 使用数据库表锁确保并发安全，同一 region 同时只能创建一个实例

### 列出 V2Ray 实例

获取所有未删除的 V2Ray 实例列表。

- **方法**：GET
- **路径**：`/api/v2ray/instances`
- **成功响应**（200）：
  ```json
  [
    {
      "id": 1,
      "uuid": "550e8400-e29b-41d4-a716-446655440000",
      "ec2_id": "i-1234567890abcdef0",
      "ec2_region": "us-east-1",
      "ec2_region_name": "美东",
      "ec2_public_ip": "203.0.113.1",
      "status": "running",
      "created_at": "2024-01-01 00:00:00",
      "updated_at": "2024-01-01 00:00:00"
    }
  ]
  ```
- **错误响应**（500）：
  ```json
  {
    "error": "failed to list instances: ..."
  }
  ```

**说明**：
- 只返回未删除的实例（is_deleted = false）
- 按创建时间倒序排列

### 获取实例详情

根据 UUID 获取单个 V2Ray 实例的详细信息。

- **方法**：GET
- **路径**：`/api/v2ray/instances/:uuid`
- **路径参数**：
  - `uuid`：实例 UUID
- **成功响应**（200）：
  ```json
  {
    "id": 1,
    "uuid": "550e8400-e29b-41d4-a716-446655440000",
    "ec2_id": "i-1234567890abcdef0",
    "ec2_region": "us-east-1",
    "ec2_region_name": "美东",
    "ec2_public_ip": "203.0.113.1",
    "status": "running",
    "created_at": "2024-01-01 00:00:00",
    "updated_at": "2024-01-01 00:00:00"
  }
  ```
- **错误响应**（404）：
  ```json
  {
    "error": "instance not found: ..."
  }
  ```
- **错误响应**（500）：
  ```json
  {
    "error": "failed to get instance: ..."
  }
  ```

### 删除 V2Ray 实例

删除指定的 V2Ray 实例。

- **方法**：DELETE
- **路径**：`/api/v2ray/instances/:uuid`
- **路径参数**：
  - `uuid`：实例 UUID
- **成功响应**（200）：
  ```json
  {
    "status": "deleting"
  }
  ```
- **错误响应**（404）：
  ```json
  {
    "error": "instance not found: ..."
  }
  ```
- **错误响应**（500）：
  ```json
  {
    "error": "failed to update status: ..."
  }
  ```

**说明**：
- 删除操作是异步的
- 实例状态会先变为 `deleting`，然后终止 EC2 实例
- 如果配置了本地 V2Ray 管理，会自动从本地配置中移除该实例

## 运行方法

1. **配置环境**：
   - 安装 Golang 1.25.5
   - 安装 MySQL
   - 配置 AWS 凭证
   - 确保本地安装了 V2Ray 服务（如果需要本地管理功能）
   - 确保当前用户有 sudo 权限（用于重启 V2Ray 服务）

2. **配置文件**：
   - 复制 `conf/conf.yaml.example` 为 `conf/conf.yaml`
   - 填写相应配置信息
   - 如需启用本地 V2Ray 管理功能，请设置 `local_config_path` 为本地 V2Ray 配置文件路径

3. **安装依赖**：
   ```bash
   go mod tidy
   ```

4. **启动服务**：
   ```bash
   go run cmd/api/main.go
   ```

   或使用自定义配置文件路径：
   ```bash
   go run cmd/api/main.go -config /path/to/config.yaml
   ```

5. **编译二进制文件**：
   ```bash
   go build -o api ./cmd/api/main.go
   ./api
   ```

## 注意事项

- 确保 AWS 凭证有足够的权限创建和管理 EC2 实例
- 确保安全组配置允许 V2Ray 访问（端口 11994）
- 首次运行时会自动创建数据库表结构
- 所有创建和删除操作都是异步的，通过状态查询获取最新状态
- 详细的操作日志会记录在 `logs/aw_backend.log` 文件中
- 如需使用本地 V2Ray 管理功能：
  - 确保本地安装了 V2Ray 服务
  - 确保当前用户有 sudo 权限
  - 确保 `local_config_path` 配置正确
  - 系统会自动备份和更新本地 V2Ray 配置文件
  - 每次配置变更后会自动重启 V2Ray 服务

## 状态说明

- **pending**：实例创建请求已接收，等待处理
- **creating**：EC2 实例正在创建，V2Ray 正在安装配置
- **running**：V2Ray 实例正常运行
- **deleting**：实例正在删除中
- **deleted**：实例已删除（EC2 实例已终止，记录保留）
- **error**：操作失败，需要手动处理