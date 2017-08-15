package main

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"log"
)

var dirPath string
var redisIp string
var redisPort string
var redisDb string
var redisPassword string
var pgUser string
var pgPassword string
var pgDbname string
var pgHost string
var pgPort string
var redisClusterName string
var redisMode string
var sentinelHosts string
var sentinelPort string

func GetDirPath() string {
	envPath := os.Getenv("GO_CONFIG_DIR")
	if envPath == "" {
		envPath = "./"
	}
	log.Println(envPath)
	return envPath
}

func GetDefaultConfig() Configuration {
	confPath := filepath.Join(dirPath, "conf.json")
	log.Println("GetDefaultConfig config path: ", confPath)
	content, operr := ioutil.ReadFile(confPath)
	if operr != nil {
		log.Fatal(operr)
	}

	defConfiguration := Configuration{}
	defErr := json.Unmarshal(content, &defConfiguration)

	if defErr != nil {
		log.Fatal("error:", defErr)
	}

	return defConfiguration
}

func LoadDefaultConfig() {

	defConfiguration := GetDefaultConfig()

	redisIp = fmt.Sprintf("%s:%s", defConfiguration.RedisIp, defConfiguration.RedisPort)
	redisPort = defConfiguration.RedisPort
	redisDb = defConfiguration.RedisDb
	redisPassword = defConfiguration.RedisPassword
	pgUser = defConfiguration.PgUser
	pgPassword = defConfiguration.PgPassword
	pgDbname = defConfiguration.PgDbname
	pgHost = defConfiguration.PgHost
	pgPort = defConfiguration.PgPort
	redisClusterName = defConfiguration.RedisClusterName
	redisMode = defConfiguration.RedisMode
	sentinelHosts = defConfiguration.SentinelHosts
	sentinelPort = defConfiguration.SentinelPort
}

func LoadConfiguration() {
	dirPath = GetDirPath()
	confPath := filepath.Join(dirPath, "custom-environment-variables.json")
	fmt.Println("InitiateRedis config path: ", confPath)

	content, operr := ioutil.ReadFile(confPath)
	if operr != nil {
		log.Println(operr)
		//LoadDefaultConfig()
	}

	envConfiguration := EnvConfiguration{}
	envErr := json.Unmarshal(content, &envConfiguration)
	if envErr != nil {
		fmt.Println("error:", envErr)
		LoadDefaultConfig()
	} else {
		defConfig := GetDefaultConfig()
		redisIp = os.Getenv(envConfiguration.RedisIp)
		redisPort = os.Getenv(envConfiguration.RedisPort)
		redisDb = os.Getenv(envConfiguration.RedisDb)
		redisPassword = os.Getenv(envConfiguration.RedisPassword)
		pgUser = os.Getenv(envConfiguration.PgUser)
		pgPassword = os.Getenv(envConfiguration.PgPassword)
		pgDbname = os.Getenv(envConfiguration.PgDbname)
		pgHost = os.Getenv(envConfiguration.PgHost)
		pgPort = os.Getenv(envConfiguration.PgPort)
		redisClusterName = os.Getenv(envConfiguration.RedisClusterName)
		redisMode = os.Getenv(envConfiguration.RedisMode)
		sentinelHosts = os.Getenv(envConfiguration.SentinelHosts)
		sentinelPort = os.Getenv(envConfiguration.SentinelPort)

		if redisIp == "" {
			redisIp = defConfig.RedisIp
		}
		if redisPort == "" {
			redisPort = defConfig.RedisPort
		}
		if redisDb == "" {
			redisDb = defConfig.RedisDb
		}
		if redisPassword == "" {
			redisPassword = defConfig.RedisPassword
		}
		if pgUser == "" {
			pgUser = defConfig.PgUser
		}
		if pgPassword == "" {
			pgPassword = defConfig.PgPassword
		}
		if pgDbname == "" {
			pgDbname = defConfig.PgDbname
		}
		if pgHost == "" {
			pgHost = defConfig.PgHost
		}
		if pgPort == "" {
			pgPort = defConfig.PgPort
		}
		if redisClusterName == "" {
			redisClusterName = defConfig.RedisClusterName
		}
		if redisMode == "" {
			redisMode = defConfig.RedisMode
		}
		if sentinelHosts == "" {
			sentinelHosts = defConfig.SentinelHosts
		}
		if sentinelPort == "" {
			sentinelPort = defConfig.SentinelPort
		}

		redisIp = fmt.Sprintf("%s:%s", redisIp, redisPort)
	}

	fmt.Println("redisMode:", redisMode)
	fmt.Println("sentinelHosts:", sentinelHosts)
	fmt.Println("sentinelPort:", sentinelPort)
	fmt.Println("redisIp:", redisIp)
	fmt.Println("redisDb:", redisDb)
}
