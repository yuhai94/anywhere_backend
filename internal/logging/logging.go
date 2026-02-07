package logging

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/google/uuid"
	"github.com/yuhai94/anywhere_backend/internal/config"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

const (
	RequestIDKey  = "request_id"
	InstanceIDKey = "instance_id"
)

var logger *zap.Logger

// Init 初始化日志系统
// 参数:
//   - logDir: 日志目录路径，用于创建日志文件
//
// 返回值:
//   - error: 错误信息，如果初始化失败
//
// 功能:
//  1. 根据配置文件选择日志格式（JSON 或开发模式）
//  2. 根据配置文件设置日志级别
//  3. 如果配置了日志文件路径，创建日志目录并设置输出路径
//  4. 构建并初始化全局日志器
func Init(logDir string) error {
	var zapConfig zap.Config

	if config.AppConfig.Logging.Format == "json" {
		zapConfig = zap.NewProductionConfig()
	} else {
		zapConfig = zap.NewDevelopmentConfig()
	}

	// Set log level
	switch config.AppConfig.Logging.Level {
	case "debug":
		zapConfig.Level = zap.NewAtomicLevelAt(zapcore.DebugLevel)
	case "info":
		zapConfig.Level = zap.NewAtomicLevelAt(zapcore.InfoLevel)
	case "warn":
		zapConfig.Level = zap.NewAtomicLevelAt(zapcore.WarnLevel)
	case "error":
		zapConfig.Level = zap.NewAtomicLevelAt(zapcore.ErrorLevel)
	case "fatal":
		zapConfig.Level = zap.NewAtomicLevelAt(zapcore.FatalLevel)
	default:
		zapConfig.Level = zap.NewAtomicLevelAt(zapcore.InfoLevel)
	}

	// Set output path
	if err := os.MkdirAll(logDir, 0755); err != nil {
		return fmt.Errorf("failed to create log directory: %v", err)
	}
	logFile := filepath.Join(logDir, "backend.log")
	zapConfig.OutputPaths = []string{logFile}
	zapConfig.ErrorOutputPaths = []string{logFile}

	// Build logger
	var err error
	logger, err = zapConfig.Build()
	if err != nil {
		return fmt.Errorf("failed to build logger: %v", err)
	}

	return nil
}

// WithRequestID 为上下文添加请求 ID
// 参数:
//   - ctx: 原始上下文
//
// 返回值:
//   - context.Context: 带有请求 ID 的新上下文
//
// 功能:
//  1. 生成一个新的 UUID 作为请求 ID
//  2. 将请求 ID 添加到上下文中
//  3. 返回带有请求 ID 的新上下文
func WithRequestID(ctx context.Context) context.Context {
	requestID := uuid.New().String()
	return context.WithValue(ctx, RequestIDKey, requestID)
}

// WithInstanceID 为上下文添加实例 ID
// 参数:
//   - ctx: 原始上下文
//   - instanceID: 实例 ID
//
// 返回值:
//   - context.Context: 带有实例 ID 的新上下文
//
// 功能:
//  1. 将实例 ID 添加到上下文中
//  2. 返回带有实例 ID 的新上下文
func WithInstanceID(ctx context.Context, instanceID string) context.Context {
	return context.WithValue(ctx, InstanceIDKey, instanceID)
}

// FromContext 从上下文创建日志器
// 参数:
//   - ctx: 上下文，可能包含请求 ID 和实例 ID
//
// 返回值:
//   - *zap.Logger: 带有上下文信息的日志器
//
// 功能:
//  1. 创建一个带有时间戳的基础日志器
//  2. 如果上下文中有请求 ID，添加到日志器
//  3. 如果上下文中有实例 ID，添加到日志器
//  4. 返回配置好的日志器
func FromContext(ctx context.Context) *zap.Logger {
	l := logger.With(zap.String("timestamp", time.Now().Format(time.RFC3339)))

	if requestID, ok := ctx.Value(RequestIDKey).(string); ok {
		l = l.With(zap.String("request_id", requestID))
	}

	if instanceID, ok := ctx.Value(InstanceIDKey).(int); ok {
		l = l.With(zap.Int("instance_id", instanceID))
	}

	return l
}

// Debug 记录调试级别的日志
// 参数:
//   - ctx: 上下文，用于传递请求范围的值
//   - format: 日志格式字符串
//   - args: 格式参数
//
// 功能:
//  1. 从上下文中获取日志器
//  2. 使用 Debugf 方法记录调试日志
func Debug(ctx context.Context, format string, args ...interface{}) {
	FromContext(ctx).Sugar().Debugf(format, args...)
}

// Info 记录信息级别的日志
// 参数:
//   - ctx: 上下文，用于传递请求范围的值
//   - format: 日志格式字符串
//   - args: 格式参数
//
// 功能:
//  1. 从上下文中获取日志器
//  2. 使用 Infof 方法记录信息日志
func Info(ctx context.Context, format string, args ...interface{}) {
	FromContext(ctx).Sugar().Infof(format, args...)
}

// Warn 记录警告级别的日志
// 参数:
//   - ctx: 上下文，用于传递请求范围的值
//   - format: 日志格式字符串
//   - args: 格式参数
//
// 功能:
//  1. 从上下文中获取日志器
//  2. 使用 Warnf 方法记录警告日志
func Warn(ctx context.Context, format string, args ...interface{}) {
	FromContext(ctx).Sugar().Warnf(format, args...)
}

// Error 记录错误级别的日志
// 参数:
//   - ctx: 上下文，用于传递请求范围的值
//   - format: 日志格式字符串
//   - args: 格式参数
//
// 功能:
//  1. 从上下文中获取日志器
//  2. 使用 Errorf 方法记录错误日志
func Error(ctx context.Context, format string, args ...interface{}) {
	FromContext(ctx).Sugar().Errorf(format, args...)
}

// Fatal 记录致命级别的日志并退出程序
// 参数:
//   - ctx: 上下文，用于传递请求范围的值
//   - format: 日志格式字符串
//   - args: 格式参数
//
// 功能:
//  1. 从上下文中获取日志器
//  2. 使用 Fatalf 方法记录致命日志并退出程序
func Fatal(ctx context.Context, format string, args ...interface{}) {
	FromContext(ctx).Sugar().Fatalf(format, args...)
}

// EC2Log 记录 EC2 操作日志
// 参数:
//   - ctx: 上下文，用于传递请求范围的值
//   - operation: EC2 操作类型
//   - region: AWS 区域
//   - instanceID: EC2 实例 ID
//   - args: 操作相关的参数
//   - err: 错误信息，如果操作失败
//
// 功能:
//  1. 从上下文中获取基础日志器
//  2. 添加 EC2 操作相关的字段
//  3. 添加操作参数
//  4. 如果有错误，记录错误日志
//  5. 如果没有错误，记录成功日志
func EC2Log(ctx context.Context, operation, region, instanceID string, args map[string]interface{}, err error) {
	l := FromContext(ctx).With(
		zap.String("operation", "ec2"),
		zap.String("ec2_operation", operation),
		zap.String("region", region),
		zap.String("instance_id", instanceID),
	)

	for k, v := range args {
		l = l.With(zap.Any(fmt.Sprintf("arg_%s", k), v))
	}

	if err != nil {
		l.With(zap.Error(err)).Error("EC2 operation failed")
	} else {
		l.Info("EC2 operation completed")
	}
}

// LogStruct 记录结构体日志
// 参数:
//   - ctx: 上下文，用于传递请求范围的值
//   - operation: 操作类型
//   - data: 要记录的结构体数据
//
// 功能:
//  1. 从上下文中获取基础日志器
//  2. 添加操作类型和结构体数据
//  3. 记录信息级别的日志
func LogStruct(ctx context.Context, operation string, data interface{}) {
	FromContext(ctx).With(
		zap.String("operation", operation),
		zap.Any("data", data),
	).Info("Struct logged")
}
