package service 

import(
	"goships/pkg/gofunc"
	"goships/pkg/logs"
	"github.com/gin-gonic/gin"
)

/**
 * 保存登陆信息
 */
func (srv *Service) GetUInfo(c *gin.Context) {
	response 			:= InitResponse()
	defer func(){
		c.JSON(HttpOk, response)
	}()
	uidStr				:= c.Query("uid")
	uid, _ 				:= gofunc.StringToInt(uidStr)
	if uid <= 0 {
		response.Code 	= 404
		return 
	}
	uInfo, err 			:= srv.Data.GetUserUser(c, uid)
	if !srv.Data.IsErrNoRows(err) {
		response.Code 	= 500
		logs.Error("GetUserUser: %v", err)
		return 
	}
	response.Result 	= map[string]interface{}{
		"uInfo" : 		uInfo,
	}
	return
}