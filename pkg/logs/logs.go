package logs

import (
	"fmt"
	"log"
	"os"
	"runtime"
	"strings"
	"sync"
	"time"
)

type localchess_log struct {
	file        *os.File
	filename    string
	level       int
	fileMaxSize int64
	fileNum     int
	sid     	int
	mux         sync.Mutex
}

const (
	LOG_DEBUG 	= iota
	LOG_INFO
	LOG_ERROR
	LOG_STRACE
	LOG_FATAL
)

var (
	lvlMap = map[int]string{0: "DEBUG", 1: "INFO", 2: "ERROR", 3: "STRACE", 4: "FATAL"}
)

var lclog *localchess_log

func Init(sid int, file string, level int){
	l := GetInstance()
	l.Initlog(sid, file, level)
}

func GetInstance() *localchess_log {
	if lclog == nil {
		lclog = new(localchess_log)
	}
	return lclog
}

func (l *localchess_log) Initlog(sid int, file string, level int) {
	var err error
	l.filename = file
	l.file, err = os.OpenFile(file, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		log.Fatal("Open File Fail : ", err)
	}
	l.level 		= level
	l.fileMaxSize 	= 1024 * 1024 * 50 		//50MB
	l.fileNum 		= 9                    	//最多4个文件，此处可以走配置
	l.sid 			= sid
}

func (l *localchess_log) write(lvl int, log_content string) {
	t := time.Now().Unix()
	if t%2 == 0 { //偶数秒判断是否创建新文件
		l.mux.Lock()
		stat, err := l.file.Stat()
		if err != nil {
			log.Println("Get Log Stat Error ", err)
			return
		}
		if stat.Size() > l.fileMaxSize {
			l.newfile()
		}
		l.mux.Unlock()
	}
	var file string
	var line int
	var funcname string
	var pc uintptr
	var ok bool
	pc, file, line, ok = runtime.Caller(2)
	if !ok {
		file = "???"
		line = 0
		funcname = "UnKnow.UnKonow"
	} else {
		funcptr := runtime.FuncForPC(pc)
		funcname = funcptr.Name()
		n := strings.LastIndexAny(file, "/")
		file = file[n+1:]
		w := strings.LastIndexAny(funcname, "/")
		funcname = funcname[w+1:]
		w = strings.LastIndexAny(funcname, ".")
		funcname = funcname[w+1:]
	}
	content := fmt.Sprintf("[%-6s %s %d] [%s:%s:%d] %s \n", lvlMap[lvl], time.Now().Format("2006-01-02 15:04:05.0000"), l.sid,
		file, funcname, line, log_content)
	l.file.WriteString(content)
}

func fileExist(path string) bool {
	_, err := os.Stat(path) //os.Stat获取文件信息
	if err != nil {
		if os.IsExist(err) {
			return true
		}
		return false
	}
	return true
}

func (l *localchess_log) newfile() {
	l.file.Close()
	newfilename := fmt.Sprintf("%s.%d", l.filename, 1)
	var oldFileName, oldNewFileName string
	for i := l.fileNum; i > 0; i-- {
		oldFileName = fmt.Sprintf("%s.%d", l.filename, i)
		if fileExist(oldFileName) && i == l.fileNum {
			os.Remove(oldFileName)
		} else if fileExist(oldFileName) {
			oldNewFileName = fmt.Sprintf("%s.%d", l.filename, i+1)
			os.Rename(oldFileName, oldNewFileName)
		}
	}
	os.Rename(l.filename, newfilename)
	var err error
	l.file, err = os.OpenFile(l.filename, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		log.Fatal("Open File Fail : ", err)
	}
}

//输出日志到文件中，DEBUG级别, format为格式化配置
func Debug(format string, v ...interface{}) {
	l := GetInstance()
	if LOG_DEBUG < l.level {
		return
	}
	str := fmt.Sprintf(format, v...)
	l.write(LOG_DEBUG, str)
}

func Info(format string, v ...interface{}) {
	l := GetInstance()
	if LOG_INFO < l.level {
		return
	}
	str := fmt.Sprintf(format, v...)
	l.write(LOG_INFO, str)
}

func Error(format string, v ...interface{}) {
	l := GetInstance()
	if LOG_ERROR < l.level {
		return
	}
	str := fmt.Sprintf(format, v...)
	l.write(LOG_ERROR, str)
}

func Strace(format string, v ...interface{}) {
	l := GetInstance()
	if LOG_STRACE < l.level {
		return
	}
	str := fmt.Sprintf(format, v...)
	l.write(LOG_STRACE, str)
}

func Fatal(format string, v ...interface{}) {
	l := GetInstance()
	if LOG_FATAL < l.level {
		return
	}
	str := fmt.Sprintf(format, v...)
	l.write(LOG_FATAL, str)
}
