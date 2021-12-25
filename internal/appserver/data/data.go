package data

import (
	"log"
	"goships/internal/appserver/config"
	"goships/pkg/database/sql"
	"goships/pkg/logs"
	"goships/pkg/cache/redis"

	"github.com/pkg/errors"
	"github.com/google/wire"
)


var ProviderSet = wire.NewSet(NewData, NewMysql)

// Data .
type Data struct {
	MainDb 		*sql.DB
	TempRds 	*redis.RedisServer
}


func NewMysql(data *config.Config) *sql.DB {
	db, err 		:= sql.NewMySQL(data.MysqlMain)
	if err != nil {
		log.Fatalf("failed opening connection to mysql: %v", err)
	}
	return db
}

// func NewRedis(config *config.Config) *redis.RedisServer {
// 	rds, err 		:= sql.NewPool(data.RedisTemp)
// 	if err != nil {
// 		log.Fatalf("failed opening connection to redis: %v", err)
// 	}
// 	return rds
// }

func NewData(mainDb *sql.DB)(*Data, func(), error) {
	d 			:= &Data{
		MainDb:  mainDb,
		// TempRds: temprds,
	}
	return d, func() {
		if err := d.MainDb.Close(); err != nil {
			logs.Error("d.MainDb.Close() err : %+v", err)
		}
	}, nil
}

func (m *Data) IsErrNoRows(err error) bool {
	return errors.Is(err, sql.ErrNoRows)
}