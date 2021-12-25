package config

import (
	"fmt"
	"log"
	"time"
	"flag"
	"os"
	"os/exec"
	"syscall"
	"strconv"
	"path/filepath"
	"gopkg.in/ini.v1"

	"goships/internal/pkg/rootpath"
	"goships/pkg/database/sql"
	"goships/pkg/cache/redis"
)

type Config struct{
	Server 			*Server
	MysqlMain 		*sql.Database
	RedisTemp 		*redis.Redis
}
type Server struct {
	Sid     		int
	Ip     			string
	RunMode      	string
	HttpPort     	string
	ReadTimeout  	time.Duration
	WriteTimeout 	time.Duration
}

const (
	ConfPath 		= "configs/common.ini"
)
var (
	daemon 			bool
	sid 			int
	Port 			int

	err 			error
	cfg 			*ini.File
	Data 			= &Config{
		Server : 		&Server{},
		MysqlMain : 	&sql.Database{},
		RedisTemp :  	&redis.Redis{},
	} 
)

/**
 * 内部初始化
 */
func Init (apptype string) {
	flag.BoolVar(&daemon, "d", false, "an int")
	flag.IntVar(&sid, "s", 0, "Server Id")
	flag.IntVar(&Port, "p", 0, "http Port")
	flag.Parse()

	if Port != 0 && Port < 8000 {
		log.Fatal("Port cannot be less than 8000")
	}
	filePath, err  	:= rootpath.GetSuccPath(ConfPath)
	if err != nil {
		log.Fatalf("ConfPath err: %s", err.Error())
	}

	// 后台执行
	Daemon(apptype)
	ReadConfig(filePath)

	// 初始化配置参数
	if Port > 8000 {
		Data.Server.HttpPort = strconv.Itoa(Port)
	}
}


func ReadConfig(sourceConfig string){
	cfg, err = ini.Load(sourceConfig)
	if err != nil {
		log.Fatalf("[error] ini.Load, fail to sourceConfig: '%s'; \nerr: %s\n", sourceConfig, err.Error())
	}
	mapTo("Server", &Data.Server)
	Data.Server.ReadTimeout 	= Data.Server.ReadTimeout * time.Second
	Data.Server.WriteTimeout 	= Data.Server.WriteTimeout * time.Second


	mapTo("MysqlMain", &Data.MysqlMain)
	mapTo("RedisTemp", &Data.RedisTemp)
}


// mapTo map section
func mapTo(section string, v interface{}) {
	if err := cfg.Section(section).MapTo(v); err != nil {
		log.Fatalf("Conf/Cfg.MapTo %s err: %s\n", section, err.Error())
	}
}


/**
 * 切换常驻模式
 */
func Daemon (apptype string){
	if !daemon {
		return
	}
	if os.Getppid() != 1 {
		osfile, err 	:= os.OpenFile(fmt.Sprintf("log/fatal.%s.log", apptype), os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
		if err != nil {
			fmt.Printf("Cannot Open Fatal log %s", err)
			return
		}
		StartTime 		:= fmt.Sprintf("Start Time [%s] \n", time.Now().Format("2006-01-02 15:04:05.0000"))
		osfile.Write([]byte(StartTime))

		filepaths, _ 	:= filepath.Abs(os.Args[0]) //将命令行参数中执行文件路径转换成可用路径
		cmd 			:= exec.Command(filepaths, os.Args[1:]...)
		cmd.Stdin 		= nil
		cmd.Stdout 		= nil
		//重定向到异常错误日志中
		cmd.Stderr 		= osfile
		cmd.SysProcAttr = &syscall.SysProcAttr{Setsid: true} //for linux
		//cmd.SysProcAttr = &syscall.SysProcAttr{HideWindow: true} //for windows

		err 			= cmd.Start()
		if err == nil {
			cmd.Process.Release()
			os.Exit(0)
		}
	}
}