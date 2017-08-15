package main

import (
	"time"
)

type Configuration struct {
	RedisIp          string
	RedisPort        string
	RedisDb          string
	RedisPassword    string
	PgUser           string
	PgPassword       string
	PgDbname         string
	PgHost           string
	PgPort           string
	RedisClusterName string
	RedisMode        string
	SentinelHosts    string
	SentinelPort     string
}

type EnvConfiguration struct {
	RedisIp          string
	RedisPort        string
	RedisDb          string
	RedisPassword    string
	PgUser           string
	PgPassword       string
	PgDbname         string
	PgHost           string
	PgPort           string
	RedisClusterName string
	RedisMode        string
	SentinelHosts    string
	SentinelPort     string
}

type SummeryDetail struct {
	Company        int
	Tenant         int
	WindowName     string
	Param1         string
	Param2         string
	MaxTime        int
	TotalCount     int
	TotalTime      int
	ThresholdValue int
	SummaryDate    time.Time
}

type ThresholdBreakDownDetail struct {
	Company        int
	Tenant         int
	WindowName     string
	Param1         string
	Param2         string
	BreakDown      string
	ThresholdCount int
	SummaryDate    time.Time
	Hour           int
}

type MetaData struct {
	EventClass      string
	EventType       string
	EventCategory   string
	WindowName      string
	Count           int
	FlushEnable     bool
	UseSession      bool
	PersistSession  bool
	ThresholdEnable bool
	ThresholdValue  int
}