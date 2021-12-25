package redis

import (
	"sync"
	"time"
	"strconv"
	"goships/pkg/logs"
	"github.com/gomodule/redigo/redis"
	"github.com/pkg/errors"
)

const (
	MAX_CONN        = 5
	TIME_OUT        = 2
	KEEP_ALIVE_TIME = 5
)
type Redis struct {
	Name        	string
	Proto        	string
	Addr         	string
	Passwd       	string
	DB       	 	int
	DialTimeout  	time.Duration
	ReadTimeout  	time.Duration
	WriteTimeout 	time.Duration
}

type RedisClient struct {
	conn        redis.Conn
	keepChan    chan bool
	IsConnected bool
	mux         sync.Mutex
}

type RedisServer struct {
	addr   	string
	spasswd string //密码
	sdb    	int  //db

	poolChan chan *RedisClient
}

var rs map[string]*RedisServer

func GetInstance(key string) *RedisServer {
	if rs == nil {
		rs = make(map[string]*RedisServer)
	}
	instance, ok := rs[key]
	if !ok {
		instance 			= new(RedisServer)
		instance.sdb 		= 0
		instance.spasswd 	= ""
		rs[key] 			= instance
	}
	return instance
}

func NewPool(c *Redis) (rs *RedisServer, err error) {
	rs 			= GetInstance(c.Name)
	rs.SetPasswd(c.Passwd)
	rs.SetDB(c.DB)
	if rs.Init(c.Addr) != 0 {
		err 		= errors.Wrap(err, "Init Redis Fail " + c.Addr)
	}
	return 
}
func (rds *RedisServer) SetPasswd(passwd string){
	rds.spasswd = passwd
}

func (rds *RedisServer) SetDB(dbindex int){
	rds.sdb = dbindex
}

func (rds *RedisServer) Init(addr string) int {
	rds.poolChan = make(chan *RedisClient, MAX_CONN)
	if newPool(addr, MAX_CONN, rds) != 0 {
		return -1
	}

	return 0
}

func StartKeepAliveCoroutine(rc *RedisClient, rds *RedisServer) {
	ticker := time.NewTicker(time.Second * 2)
	defer ticker.Stop()
	for {
		select {
		case <-rc.keepChan:
			return
		case <-ticker.C:

			rc.mux.Lock()
			if rc.IsConnected == false {
				rc.Reconnect(rds.addr, rds.spasswd, rds.sdb)
			} else {
				ret, err := redis.String(rc.conn.Do("PING"))
				if err != nil || ret != "PONG" {
					rc.conn.Close()
					rc.IsConnected = false
					rc.Reconnect(rds.addr, rds.spasswd, rds.sdb)
				}
			}
			rc.mux.Unlock()
		}
	}
}

func (rc *RedisClient) Reconnect(addr string, password string, dbindex int) {
	timeout := time.Second * time.Duration(TIME_OUT)
	conn, err := redis.Dial("tcp", addr, redis.DialConnectTimeout(timeout),
		redis.DialConnectTimeout(timeout),
		redis.DialConnectTimeout(timeout),
		redis.DialPassword(password),
		redis.DialDatabase(dbindex),
	)
	if err != nil {
		logs.Error("Redis Reconnect Fail: "+addr)
		return
	}
	rc.conn 		= conn
	rc.IsConnected = true
	logs.Info("Redis Reconnect succ: "+addr)
}

func newPool(addr string, max_con int, rds *RedisServer) int {
	timeout := time.Second * time.Duration(TIME_OUT)
	var connList []redis.Conn
	for i := 0; i < max_con; i++ {
		//conn, err := redis.DialTimeout("tcp", addr, timeout, timeout, timeout)
		conn, err := redis.Dial("tcp", addr, redis.DialConnectTimeout(timeout),
			redis.DialConnectTimeout(timeout),
			redis.DialConnectTimeout(timeout),
			redis.DialPassword(rds.spasswd),
			redis.DialDatabase(rds.sdb),
		)
		if err != nil {
			logs.Error("Connect Redis Fail Err=%s", err)
			for j := 0; j < len(connList); j++ {
				connList[j].Close()
			}
			return -1
		}
		connList = append(connList, conn)
	}
	for j := 0; j < len(connList); j++ {
		rc := new(RedisClient)
		rc.conn = connList[j]
		rc.keepChan = make(chan bool)
		rc.IsConnected = true
		go StartKeepAliveCoroutine(rc, rds)
		rds.poolChan <- rc
	}
	return 0
}

func (rds *RedisServer) GetConn() *RedisClient {
	ticker := time.NewTicker(time.Second * 2)
	failTryTimes := 0
	defer ticker.Stop()
	for {
		select {
		//从通道中获取连接资源
		case connRes, ok := <-rds.poolChan:
			{
				if !ok {
					return nil
				}
				//判断连接中的时间，如果超时，则关闭
				//继续获取
				connRes.mux.Lock()
				if connRes.IsConnected == false && failTryTimes < 3 {
					logs.Error("Get Redis Fail This Redis Socket Closed , Try Other Redis Socket")
					connRes.mux.Unlock()
					rds.poolChan <- connRes //失败后继续推入到通道中
					failTryTimes++
					continue
				} else if connRes.IsConnected == false && failTryTimes >= 3 {
					logs.Error("Max Try Get Redis Times Return Nil")
					connRes.mux.Unlock()
					rds.poolChan <- connRes //失败后继续推入到通道中
					return nil
				}
				failTryTimes = 0
				return connRes
			}
		case <-ticker.C: //获取超时，返回nil
			{
				return nil
			}
		}
	}

	return nil
}

func (rds *RedisServer) Release(rc *RedisClient) {

	rc.mux.Unlock()

	rds.poolChan <- rc

}

func (rds *RedisServer) SetByString(key string, value string) bool {
	rc := rds.GetConn()
	if rc == nil {
		return false
	}
	ret, err := redis.Bool(rc.conn.Do("Set", key, value))
	rds.Release(rc)
	if err != nil || !ret {
		return false
	}
	return true
}

//设置过期时间
//expire 为多少秒内过期时间，非绝对时间
func (rds *RedisServer) Setex(key string, value string, expire int) bool {
	rc := rds.GetConn()
	if rc == nil {
		return false
	}
	// expire_str 	:= strconv.Itoa(expire)
	_, err 	:= rc.conn.Do("SETEX", key, expire, value)
	rds.Release(rc)
	if err != nil {
		return false
	}
	return true
}

//（SET if Not eXists） 命令在指定的 key 不存在时，为 key 设置指定的值; 存在则false
func (rds *RedisServer) Setnx(key string, value string) bool {
	rc := rds.GetConn()
	if rc == nil {
		return false
	}
	ret, err := redis.Bool(rc.conn.Do("SETNX", key, value))
	rds.Release(rc)
	if err != nil || !ret {
		return false
	}
	return true
}

func (rds *RedisServer) Del(key string) bool {
	rc := rds.GetConn()
	if rc == nil {
		return false
	}
	ret, err := redis.Bool(rc.conn.Do("DEL", key))
	rds.Release(rc)
	if err != nil || !ret {
		return false
	}
	return true
}

func (rds *RedisServer) SetByNumber(key string, value int) bool {
	rc := rds.GetConn()
	if rc == nil {
		return false
	}
	ret, err := redis.Bool(rc.conn.Do("Set", key, value))
	rds.Release(rc)
	if err != nil || !ret {
		return true
	}
	return false
}

func (rds *RedisServer) GetSets(key string) []string {
	rc := rds.GetConn()
	if rc == nil {
		return nil
	}
	ret, err := redis.Strings(rc.conn.Do("SMEMBERS", key))
	rds.Release(rc)
	if err != nil {
		return nil
	}
	return ret
}

func (rds *RedisServer) SAddMember(key string, val interface{}) bool {
	var str string
	switch val := val.(type) {
	case int:
		str = strconv.Itoa(val)
	case []byte:
		str = string(val)
	case nil:
		return false
	case string:
		str = val
	default:
		return false
	}
	rc := rds.GetConn()
	if rc == nil {
		return false
	}
	ret, err := redis.Bool(rc.conn.Do("SADD", key, str))
	rds.Release(rc)
	if err != nil {
		return false
	}
	return ret
}

//批量写入
func (rds *RedisServer) SMulitiAddMember(params ...string) bool {
	rc := rds.GetConn()
	if rc == nil {
		return false
	}
	s := make([]interface{}, len(params))
	for i, v := range params {
		s[i] = v
	}
	ret, err := redis.Bool(rc.conn.Do("SADD", s...))
	rds.Release(rc)
	if err != nil {
		return false
	}
	return ret
}

//删除元素
func (rds *RedisServer) SRem(key string, val interface{}) bool {
	var str string
	switch val := val.(type) {
	case int:
		str = strconv.Itoa(val)
	case []byte:
		str = string(val)
	case nil:
		return false
	case string:
		str = val
	default:
		return false
	}
	rc := rds.GetConn()
	if rc == nil {
		return false
	}
	ret, err := redis.Bool(rc.conn.Do("SREM", key, str))
	rds.Release(rc)
	if err != nil {
		return false
	}
	return ret
}

func (rds *RedisServer) SExist(key string, val string) bool {
	rc := rds.GetConn()
	if rc == nil {
		return false
	}
	ret, err := redis.Bool(rc.conn.Do("SISMEMBER", key, val))
	rds.Release(rc)
	if err != nil {
		return false
	}
	return ret
}

/*
http://redis.cn/commands/hset.html
返回值
1如果field是一个新的字段
0如果field原来在map里面已经存在
*/
func (rds *RedisServer) HSet(key string, sub_key string, val string) bool {
	if key == "" || sub_key == "" {
		return false
	}
	rc := rds.GetConn()
	if rc == nil {
		return false
	}
	_, err := redis.Int(rc.conn.Do("HSET", key, sub_key, val))
	rds.Release(rc)
	if err != nil {
		logs.Error("hset err (%s) key=%s sub_key=%s val=%s", err, key, sub_key, val)
		return false
	}
	return true
}


func (rds *RedisServer) HDel(key string, sub_key string) bool {
	rc := rds.GetConn()
	if rc == nil {
		return false
	}
	ret, err := redis.Bool(rc.conn.Do("HDel", key, sub_key))
	rds.Release(rc)
	if err != nil || !ret {
		return false
	}
	return true
}

func (rds *RedisServer) HMSet(key string, vals map[string]string) bool {
	if key == "" {
		return false
	}
	rc := rds.GetConn()
	if rc == nil {
		return false
	}
	_, err := rc.conn.Do("HMSET", redis.Args{}.Add(key).AddFlat(vals)...)
	rds.Release(rc)
	if err != nil {
		logs.Error("hmset err (%s) key=%s val=%#v", err, key, vals)
		return false
	}
	return true
}

func (rds *RedisServer) HGet(key string, sub_key string) string {
	if key == "" || sub_key == "" {
		return ""
	}
	rc := rds.GetConn()
	if rc == nil {
		return ""
	}
	ret, err := redis.String(rc.conn.Do("HGET", key, sub_key))
	rds.Release(rc)
	if err != nil {
		return ""
	}
	return ret
}

//批量获取
func (rds *RedisServer) HMGet(key string, sub_key []interface{}) []string {
	if key == "" || len(sub_key) == 0 {
		return nil
	}
	rc := rds.GetConn()
	if rc == nil {
		return nil
	}
	keys 				:= []interface{}{key}
	keys 				= append(keys, sub_key...)
	ret, err := redis.Strings(rc.conn.Do("HMGET", keys...))
	rds.Release(rc)
	if err != nil {
		return nil
	}
	return ret
}

func (rds *RedisServer) HGetAll(key string) map[string]string {
	if key == "" {
		return nil
	}
	rc := rds.GetConn()
	if rc == nil {
		return nil
	}
	ret, err := redis.StringMap(rc.conn.Do("HGETALL", key))
	rds.Release(rc)
	if err != nil {
		return nil
	}
	return ret
}

func (rds *RedisServer) Expire(key string, expire int) bool {
	if key == "" || expire <= 0 {
		return false
	}
	rc := rds.GetConn()
	if rc == nil {
		return false
	}
	ret, err := redis.Bool(rc.conn.Do("EXPIRE", key, expire))
	rds.Release(rc)
	if err != nil {
		return false
	}
	return ret
}

//批量获取
func (rds *RedisServer) MGet(keys []interface{}) []string {
	if len(keys) == 0 {
		return nil
	}
	rc := rds.GetConn()
	if rc == nil {
		return nil
	}
	ret, err := redis.Strings(rc.conn.Do("MGET", keys...))
	rds.Release(rc)
	if err != nil {
		return nil
	}
	return ret
}

func (rds *RedisServer) GetValReturnString(key string) string {
	rc := rds.GetConn()
	if rc == nil {
		return ""
	}
	ret, err := redis.String(rc.conn.Do("Get", key))
	rds.Release(rc)
	if err != nil {
		return ""
	}
	return ret
}

func (rds *RedisServer) Incrby(key string, val string) int64 {
	rc 				:= rds.GetConn()
	if rc == nil {
		return 0
	}
	ret, err 		:= redis.Int64(rc.conn.Do("Incrby", key, val))
	rds.Release(rc)
	if err != nil {
		return 0
	}
	return ret
}
func (rds *RedisServer) IncrbyFloat(key string, val float64) float64 {
	rc 				:= rds.GetConn()
	if rc == nil {
		return 0
	}
	ret, err 		:= redis.Float64(rc.conn.Do("INCRBYFLOAT", key, val))
	rds.Release(rc)
	if err != nil {
		return 0
	}
	return ret
}

func (rds *RedisServer) Llen(key string) int {
	if key == "" {
		return 0
	}
	rc := rds.GetConn()
	if rc == nil {
		return 0
	}
	ret, err := redis.Int(rc.conn.Do("LLEN", key))
	rds.Release(rc)
	if err != nil {
		return 0
	}
	return ret
}

func (rds *RedisServer) LPop(key string) string {
	if key == "" {
		return ""
	}
	rc := rds.GetConn()
	if rc == nil {
		return ""
	}
	ret, err := redis.String(rc.conn.Do("LPOP", key))
	rds.Release(rc)
	if err != nil {
		return ""
	}
	return ret
}
func (rds *RedisServer) RPush(key string, data string) bool {
	if key == "" {
		return false
	}
	rc := rds.GetConn()
	if rc == nil {
		return false
	}
	ret, err := redis.Bool(rc.conn.Do("RPUSH", key, data))
	rds.Release(rc)
	if err != nil {
		return false
	}
	return ret
}

func (rds *RedisServer) RPop(key string) string {
	if key == "" {
		return ""
	}
	rc := rds.GetConn()
	if rc == nil {
		return ""
	}
	ret, err := redis.String(rc.conn.Do("RPOP", key))
	rds.Release(rc)
	if err != nil {
		return ""
	}
	return ret
}

func (rds *RedisServer) LPush(key string, data string) bool {
	if key == "" {
		return false
	}
	rc := rds.GetConn()
	if rc == nil {
		return false
	}
	ret, err := redis.Bool(rc.conn.Do("LPUSH", key, data))
	rds.Release(rc)
	if err != nil {
		return false
	}
	return ret
}

func (rds *RedisServer) LRange(key string, start int, end int) []string {
	if key == "" {
		return nil
	}
	rc := rds.GetConn()
	if rc == nil {
		return nil
	}
	ret, err := redis.Strings(rc.conn.Do("LRANGE", key, start, end))
	rds.Release(rc)
	if err != nil {
		return nil
	}
	return ret
}

func (rds *RedisServer) HKeys(key string) []string {
	if key == "" {
		return nil
	}
	rc := rds.GetConn()
	if rc == nil {
		return nil
	}
	ret, err := redis.Strings(rc.conn.Do("HKEYS", key))
	rds.Release(rc)
	if err != nil {
		return nil
	}
	return ret
}

func (rds *RedisServer) TTL(key string) int {
	if key == "" {
		return 0
	}
	rc := rds.GetConn()
	if rc == nil {
		return 0
	}
	ret, err := redis.Int(rc.conn.Do("TTL", key))
	rds.Release(rc)
	if err != nil {
		return 0
	}
	return ret
}

func (rds *RedisServer) SetBit(key string, sub_key int, val int) bool {
	if key == "" || sub_key == 0 {
		return false
	}
	rc := rds.GetConn()
	if rc == nil {
		return false
	}
	_, err := redis.Int(rc.conn.Do("SETBIT", key, sub_key, val))
	rds.Release(rc)
	if err != nil {
		logs.Error("hset err (%s) key=%s sub_key=%s val=%s", err, key, sub_key, val)
		return false
	}
	return true
}

func (rds *RedisServer) GetBit(key string, sub_key int) int64 {
	if key == "" || sub_key == 0 {
		return 0
	}
	rc := rds.GetConn()
	if rc == nil {
		return 0
	}
	// ret, err := redis.Int64(rc.conn.Do("GETBIT", key, sub_key))
	ret, err := redis.Int64(rc.conn.Do("GETBIT", key, sub_key))
	rds.Release(rc)
	if err != nil {
		return 0
	}
	return ret
}

func (rds *RedisServer) BitOp(params ...string) bool {
	rc 				:= rds.GetConn()
	if rc == nil {
		return false
	}
	s 				:= make([]interface{}, len(params))
	for i, v := range params {
		s[i] 		= v
	}
	ret, err 		:= redis.Bool(rc.conn.Do("BITOP", s...))
	rds.Release(rc)
	if err != nil {
		return false
	}
	return ret
}

func (rds *RedisServer) BitCount(key string) int64 {
	rc := rds.GetConn()
	if rc == nil {
		return 0
	}
	ret, err := redis.Int64(rc.conn.Do("BITCOUNT", key))
	rds.Release(rc)
	if err != nil {
		return 0
	}
	return ret
}

func (rds *RedisServer) Keys(key string) []string {
	if key == "" {
		return nil
	}
	rc := rds.GetConn()
	if rc == nil {
		return nil
	}
	ret, err := redis.Strings(rc.conn.Do("KEYS", key))
	rds.Release(rc)
	if err != nil {
		return nil
	}
	return ret
}

func (rds *RedisServer) ZAdd(key string, val interface{}, sub_key string) bool {

	if key == "" || sub_key == "" {
		return false
	}
	rc := rds.GetConn()
	if rc == nil {
		return false
	}
	_, err := redis.Int(rc.conn.Do("ZADD", key, val, sub_key))
	rds.Release(rc)
	if err != nil {
		logs.Error("ZADD err (%s) key=%s sub_key=%s val=%s", err, key, sub_key, val)
		return false
	}
	return true
}

func (rds *RedisServer) ZRevRangeWithScores(key string, start, end int) map[string]string {
	if key == "" {
		return nil
	}
	rc := rds.GetConn()
	if rc == nil {
		return nil
	}
	ret, err := redis.StringMap(rc.conn.Do("ZREVRANGE", key, start, end, "WITHSCORES"))
	// ret, err := redis.Values(rc.conn.Do("ZREVRANGE", key, start, end, "WITHSCORES"))
	rds.Release(rc)
	if err != nil {
		logs.Error("ZREVRANGE err (%s) key=%s start=%d end=%d", err, key, start, end)
		return nil
	}
	return ret
}

func (rds *RedisServer) ZScoreReturnString(key string, sub_key string, defval string) string {
	rc := rds.GetConn()
	if rc == nil {
		return defval
	}
	ret, err := redis.String(rc.conn.Do("ZSCORE", key, sub_key))
	rds.Release(rc)
	if err != nil {
		return defval
	}
	if ret == "" {
		return defval
	}
	return ret
}