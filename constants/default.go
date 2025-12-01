package constants

import "time"

const (
	SuperadminUsername     = "superadmin"
	SuperadminDefaultEmail = "superadmin@internal.com"
	SuperadminID           = "00000000-0000-0000-0000-000000000001"
)

const (
	HTTPServerDefaultPort      = 8080
	HTTPDefaultRequestTimeout  = 10
	HTTPDefaultGraceWaitPeriod = 10 * time.Second
)

const (
	ProfilingServerDefaultPort = 6060
)

const (
	DefaultDBTimeout             = 10 * time.Second
	DefaultDBReadTimeout         = 10 * time.Second
	DefaultDBWriteTimeout        = 10 * time.Second
	DefaultDBMaxIdleConn     int = 100
	DefaultDBMaxOpenConn     int = 100
	DefaultDBConnMaxLifetime     = 30 * time.Minute
	DefaultDBConnMaxIdleTime     = 60 * time.Minute
)
