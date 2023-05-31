package config

import "time"

type RunConfig struct {
	HttpPort                      int
	ServeHttp                     bool
	ScheduleWindow                time.Duration
	RunnerWindow                  time.Duration
	CleanupWindow                 time.Duration
	CockroachdbUri                string
	CockroachdbMaxIdleConnections int
	CockroachdbMaxOpenConnections int
	CockroachdbConnMaxLifetime    time.Duration
	NotifoBaseUrl                 string
	NotifoApiKey                  string
	NotifoAppId                   string
}

var AppConfig RunConfig
