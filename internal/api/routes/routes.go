package routes

import (
	"github.com/gin-gonic/gin"
	"github.com/yuhai94/anywhere_backend/internal/api/handlers"
)

// SetupRoutes 设置 API 路由
// 参数:
//   - router: Gin 路由器实例
//   - v2rayHandler: V2RayHandler 实例，用于处理 V2Ray 相关的请求
//
// 功能:
//  1. 创建 API 路由组
//  2. 为 V2Ray 相关操作设置路由
//     - GET /api/v2ray/regions: 获取支持的区域列表
//     - POST /api/v2ray/instances: 创建实例
//     - GET /api/v2ray/instances: 获取实例列表
//     - GET /api/v2ray/instances/:id: 获取实例详情
//     - DELETE /api/v2ray/instances/:id: 删除实例
func SetupRoutes(router *gin.Engine, v2rayHandler *handlers.V2RayHandler) {
	api := router.Group("/api")
	{
		v2ray := api.Group("/v2ray")
		{
			v2ray.GET("/regions", v2rayHandler.ListRegions)
			v2ray.POST("/instances", v2rayHandler.CreateInstance)
			v2ray.GET("/instances", v2rayHandler.ListInstances)
			v2ray.GET("/instances/:uuid", v2rayHandler.GetInstance)
			v2ray.DELETE("/instances/:uuid", v2rayHandler.DeleteInstance)
		}
	}
}
