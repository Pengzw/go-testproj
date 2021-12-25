package data

import (
	"fmt"
	"context"
	"github.com/pkg/errors"
	"goships/pkg/database/sql"
)

type UserUser struct {
	Uid       			int `json:"uid"`
	Appid       		int `json:"appid"`
	Tel 				string `json:"tel"`
	Nickname 			string `json:"nickname"`
	Headimgurl 			string `json:"headimgurl"`
	Sex 				int `json:"sex"`
}

const (
	_userShard 			= 10 // 分表十张
	_DelUserUser 		= "DELETE from dd_user%s where uid=?"
	_GetUserUser 		= "SELECT `uid`, `appid`, `tel`, `nickname`, `headimgurl`, `sex` from dd_user%s where uid=?"
	_UpdUserUser 		= "UPDATE dd_user%s set %s where uid = ?"
)

// userTableIndex return index by uid
func userTableIndex(uid int) string {
	return fmt.Sprintf("%d", uid%_userShard)
}

func (m *Data) GetUserUser(c context.Context, uid int) (result *UserUser, err error) {
	result 				= &UserUser{}

	err 				= m.MainDb.QueryRow(c, fmt.Sprintf(_GetUserUser, userTableIndex(uid)), uid).Scan(
		&result.Uid, &result.Appid, &result.Tel, &result.Nickname, &result.Headimgurl, &result.Sex)
	if err != sql.ErrNoRows {
		err 			= errors.Wrap(err, "QueryRow _GetUserUser fail")
	}
	return
}
