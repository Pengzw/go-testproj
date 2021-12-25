package server

import (
	"net/http"
	"github.com/gin-gonic/gin"

	"goships/internal/appserver/service"
)

// InitRouter initialize routing information
func InitRouter(srv *service.Service) *gin.Engine {
	// 创建一个不包含中间件的路由器
	router := gin.New()

	// router.Use(gin.Logger())
	// router.Use(middleware.Recovery())
	// // 链路追踪
	// router.Use(middleware.Tracing())

	// 设置文件上传大小限制，默认是32m
	router.MaxMultipartMemory = 32 << 20  // 64 MiB

	router.GET("/user/info", srv.GetUInfo)

	router.GET("/", func(c *gin.Context) {
		c.String(http.StatusOK, "server test")
	})
	return router
}
