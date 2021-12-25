package server

import(
	// "log"
	// "time"
	// "context"
	// "os"
	// "os/signal"
	// "syscall"
	"net/http"
	"goships/internal/appserver/config"
	"goships/internal/appserver/service"
	"github.com/gin-gonic/gin"
	"github.com/google/wire"
)
var (
	ProviderSet 	= wire.NewSet(NewHttp)
)

func NewHttp(config *config.Config, srv *service.Service) (server *http.Server) {
	gin.SetMode(config.Server.RunMode)

	return &http.Server{
		// 监听的地址
		Addr:           config.Server.Ip + ":" + config.Server.HttpPort,
		// http句柄，实质为ServeHTTP，用于处理程序响应HTTP请求
		Handler:        InitRouter(srv),
		// 允许读取的最大时间
		ReadTimeout:    config.Server.ReadTimeout,
		// 允许写入的最大时间
		WriteTimeout:   config.Server.WriteTimeout,
		// 请求头的最大字节数
		MaxHeaderBytes: 1 << 20,
	}
}
