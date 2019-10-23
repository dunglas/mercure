package config

import (
	_ "github.com/joho/godotenv/autoload"
	"github.com/spf13/viper"
	"log"
	"os"
	"path"
)

func init(){
	rootPath,_ := os.Getwd()
	viper.AutomaticEnv()
	viper.AddConfigPath(".")
	viper.AddConfigPath( path.Join(rootPath,".."))
	err := viper.ReadInConfig()
	if err != nil {
		log.Println("Loading config from enviroment variables..")
	}
}

func Get(key string) interface{}{
	return viper.Get(key)
}

func GetString(key string) string{
	value := Get(key)
	if value == nil{
		return ""
	}
	return Get(key).(string)
}
