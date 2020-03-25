package boot

import (
	"fmt"
	"github.com/go-redis/redis"
	"github.com/spf13/viper"
	"os"
	"sync"
)

var (
	AkaRds *redis.Client
	wn     sync.Once
	chapterKeyPrefix = "novels_"
)

func init() {
	workPath,_ := os.Getwd()
	viper.AddConfigPath(workPath+"/conf/")
	viper.SetConfigName("zigzag")
	viper.SetConfigType("yml")

	err := viper.ReadInConfig()
	if err != nil {
		panic(err)
	}
}

func GetNovelsChapterKey(id string) (string) {
	return chapterKeyPrefix + id
}

func GetRgxPoint() (string) {
	return chapterKeyPrefix + "*"
}

// connect redis
func BootRds() error {
	redisConf:=viper.GetStringMapString("redis")
	addr:=fmt.Sprintf("%s:%s",redisConf["host"],redisConf["port"])

	wn.Do(func() {
		AkaRds = redis.NewClient(&redis.Options{
			Addr:     addr,
			Password: redisConf["password"],
			DB:       viper.GetInt("redis.db"),
		})
	})
	return AkaRds.Ping().Err()
}
