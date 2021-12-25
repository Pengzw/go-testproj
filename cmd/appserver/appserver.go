package main

import (
	"fmt"
	"log"
	"time"

	"os"
	"os/signal"
	"syscall"
	"context"
	"net/http"
	"github.com/pkg/errors"
	"golang.org/x/sync/errgroup"
	"goships/internal/appserver/config"
	"goships/internal/appserver/service"
	"goships/internal/appserver/server"
	"goships/internal/appserver/data"
	"goships/pkg/logs"
)

func main() {
	config.Init("appserver")
	logs.Init(config.Data.Server.Sid, fmt.Sprintf("log/appserver.%d.debug.log", config.Data.Server.Sid), logs.LOG_DEBUG)// LOG_FATAL最高错误级别
	log.Printf("Server: %+v \n", config.Data.Server)

	dataData, cleanup, err := data.NewData(data.NewMysql(config.Data))
	if err != nil {
		panic(err)
		return 
	}
	defer cleanup()

	server 				:= server.NewHttp(config.Data, service.NewService(dataData))
	StartServer(server)
	// cleanup, err := initApp(config.Data)
	// if err != nil {
	// 	panic(err)
	// }
	// defer cleanup()
	// log.Println("Server cleanup")
	// // // start and wait for stop signal
	// // if err := app.Run(); err != nil {
	// // 	panic(err)
	// // }
}

func StartServer(server *http.Server) (err error) {
	log.Println("Server start")
	eg, ctx 		:= errgroup.WithContext(context.Background())

	// 开始监听
	eg.Go(func() (err error) {
		logs.Info("[info] start http server listening %s \n", server.Addr)
		return server.ListenAndServe()
	})

	// 捕获到 os 退出信号将会退出
	eg.Go(func() error {
		quit 		:= make(chan os.Signal, 0)
		signal.Notify(quit, syscall.SIGHUP, syscall.SIGINT, syscall.SIGTERM, syscall.SIGQUIT, os.Interrupt, os.Kill, syscall.SIGUSR1, syscall.SIGUSR2)

		select {
		case <-ctx.Done():
			return ctx.Err()
		case sig := <-quit:
			return errors.Errorf("quit_ch: get os signal: %v", sig)
		}

		timeoutCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()

		log.Println("Server shutdown")
		return server.Shutdown(timeoutCtx)
	})
	err 			= eg.Wait()
	logs.Debug("errgroup exit: %+v", err)
	log.Printf("errgroup exit: %+v\n", err)
	log.Println("Server end")
	return
}
// // initApp init kratos application.
// func initApp(confServer *conf.Server, registry *conf.Registry, confData *conf.Data, logger log.Logger, tracerProvider *trace.TracerProvider) (*kratos.App, func(), error) {
// 	client := data.NewEntClient(confData, logger)
// 	dataData, cleanup, err := data.NewData(client, logger)
// 	if err != nil {
// 		return nil, nil, err
// 	}
// 	beerRepo := data.NewBeerRepo(dataData, logger)
// 	beerUseCase := biz.NewBeerUseCase(beerRepo, logger)
// 	catalogService := service.NewCatalogService(beerUseCase, logger)
// 	grpcServer := server.NewGRPCServer(confServer, logger, tracerProvider, catalogService)
// 	registrar := server.NewRegistrar(registry)
// 	app := newApp(logger, grpcServer, registrar)
// 	return app, func() {
// 		cleanup()
// 	}, nil
// }
