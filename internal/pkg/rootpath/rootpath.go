package rootpath

import(
	"os"
	"fmt"
	"errors"
	"path/filepath"
	"goships/pkg/gofunc"
)


func GetSuccPath(confPath string) (rootPath string, err error) {
    // 用户目录
	WorkPath, _ 		:= os.Getwd()
	roomPath 			:= filepath.Join(WorkPath, confPath)
	if gofunc.FileExists(roomPath) {
		rootPath 		= roomPath
		return
	}

	// 程序目录
	AppPath, _ 			:= filepath.Abs(filepath.Dir(os.Args[0]))
	roomPath 			= filepath.Join(AppPath, confPath)
	if gofunc.FileExists(roomPath) {
		rootPath 		= roomPath
		return 
	}
	err 				= errors.New(fmt.Sprintf("WorkPath:%s, AppPath:%s, SetPath fail err:%s", WorkPath, AppPath, confPath))
	return 
}