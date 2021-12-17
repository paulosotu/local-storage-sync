package models

import (
	"flag"
)

const (
	exitCodeErr         = 1
	exitCodeInterrupt   = 0
	defaultLogLevel     = "INFO"
	defaultTimerTick    = 10
	defaultDataDir      = "/data-sync"
	defaultNodeName     = "raspberry2"
	defaultRunInCluster = false
	defaultSelector     = ""
	defaultDSAppName    = "storage-sync"
)

type Config struct {
	ExitCodeErr       int
	ExitCodeInterrupt int
	TimerTick         int
	DataDir           string
	NodeName          string
	DSAppName         string
	LogLevel          string
	Selector          string
	RunInCluster      bool
}

func NewConfigFromArgs() *Config {
	ret := &Config{}

	flag.StringVar(&ret.DSAppName, "app", defaultDSAppName, "k8 App deamon set app name")
	flag.StringVar(&ret.Selector, "s", defaultSelector, "Label selector")
	flag.StringVar(&ret.LogLevel, "l", defaultLogLevel, "Log level")
	flag.IntVar(&ret.TimerTick, "t", defaultTimerTick, "RSync refresh tick")
	flag.StringVar(&ret.DataDir, "d", defaultDataDir, "Nodes root data directory")
	flag.StringVar(&ret.NodeName, "n", defaultNodeName, "Host node name")
	flag.BoolVar(&ret.RunInCluster, "c", defaultRunInCluster, "True when running inside a k8 cluster")

	flag.Parse()

	ret.ExitCodeErr = exitCodeErr
	ret.ExitCodeInterrupt = exitCodeInterrupt

	return ret
}
