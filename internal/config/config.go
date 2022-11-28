package config

import "time"

type RunConfig struct {
	HttpPort       int
	ServeHttp      bool
	ScheduleWindow time.Duration
	RunnerWindow   time.Duration
	CleanupWindow  time.Duration
	CockroachdbUri string
}

var AppConfig RunConfig
