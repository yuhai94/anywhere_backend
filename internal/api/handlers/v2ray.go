package handlers

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/yuhai94/anywhere_backend/internal/logging"
	"github.com/yuhai94/anywhere_backend/internal/service"
)

type V2RayHandler struct {
	service *service.V2RayService
}

// NewV2RayHandler 创建一个新的 V2RayHandler 实例
// 参数:
//   - service: V2RayService 实例，用于处理 V2Ray 相关的业务逻辑
//
// 返回值:
//   - *V2RayHandler: 新创建的 V2RayHandler 实例
func NewV2RayHandler(service *service.V2RayService) *V2RayHandler {
	return &V2RayHandler{
		service: service,
	}
}

type CreateInstanceRequest struct {
	Region string `json:"region" binding:"required"`
}

type CreateInstanceResponse struct {
	UUID   string `json:"uuid"`
	Status string `json:"status"`
}

type DeleteInstanceResponse struct {
	Status string `json:"status"`
}

// CreateInstance 处理创建 V2Ray 实例的 HTTP 请求
// 参数:
//   - c: Gin 上下文，用于处理 HTTP 请求和响应
//
// 功能:
//  1. 解析请求体中的区域信息
//  2. 调用服务层创建实例
//  3. 返回创建的实例 UUID 和状态
func (h *V2RayHandler) CreateInstance(c *gin.Context) {
	ctx := logging.WithRequestID(c.Request.Context())

	var req CreateInstanceRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	logging.Info(ctx, "Creating instance in region %s", req.Region)

	uuid, err := h.service.CreateInstance(ctx, req.Region)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, CreateInstanceResponse{
		UUID:   uuid,
		Status: "pending",
	})
}

// ListInstances 处理获取所有 V2Ray 实例列表的 HTTP 请求
// 参数:
//   - c: Gin 上下文，用于处理 HTTP 请求和响应
//
// 功能:
//  1. 调用服务层获取实例列表
//  2. 返回实例列表
func (h *V2RayHandler) ListInstances(c *gin.Context) {
	ctx := logging.WithRequestID(c.Request.Context())

	instances, err := h.service.ListInstances(ctx)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, instances)
}

// GetInstance 处理获取指定 V2Ray 实例详情的 HTTP 请求
// 参数:
//   - c: Gin 上下文，用于处理 HTTP 请求和响应
//
// 功能:
//  1. 解析路径参数中的实例 ID
//  2. 调用服务层获取实例详情
//  3. 返回实例详情
func (h *V2RayHandler) GetInstance(c *gin.Context) {
	ctx := logging.WithRequestID(c.Request.Context())

	uuid := c.Param("uuid")
	instance, err := h.service.GetInstance(ctx, uuid)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, instance)
}

// DeleteInstance 处理删除指定 V2Ray 实例的 HTTP 请求
// 参数:
//   - c: Gin 上下文，用于处理 HTTP 请求和响应
//
// 功能:
//  1. 解析路径参数中的实例 ID
//  2. 调用服务层删除实例
//  3. 返回删除状态
func (h *V2RayHandler) DeleteInstance(c *gin.Context) {
	ctx := logging.WithRequestID(c.Request.Context())

	uuid := c.Param("uuid")
	if err := h.service.DeleteInstance(ctx, uuid); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, DeleteInstanceResponse{
		Status: "deleting",
	})
}

// ListRegions 处理获取支持的 AWS 区域列表的 HTTP 请求
// 参数:
//   - c: Gin 上下文，用于处理 HTTP 请求和响应
//
// 功能:
//  1. 调用服务层获取支持的区域列表
//  2. 返回区域列表
func (h *V2RayHandler) ListRegions(c *gin.Context) {
	ctx := logging.WithRequestID(c.Request.Context())

	regions := h.service.ListRegions(ctx)
	c.JSON(http.StatusOK, regions)
}
