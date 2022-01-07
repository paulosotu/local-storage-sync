package models

import (
	"flag"
)

const (
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
	exitCodeInterrupt int
	timerTick         int
	dataDir           string
	nodeName          string
	dsAppName         string
	logLevel          string
	selector          string
	runInCluster      bool
}

func NewConfigFromArgs() *Config {
	ret := &Config{}

	flag.StringVar(&ret.dsAppName, "app", defaultDSAppName, "k8 App deamon set app name")
	flag.StringVar(&ret.selector, "s", defaultSelector, "Label selector")
	flag.StringVar(&ret.logLevel, "l", defaultLogLevel, "Log level")
	flag.IntVar(&ret.timerTick, "t", defaultTimerTick, "RSync refresh tick")
	flag.StringVar(&ret.dataDir, "d", defaultDataDir, "Nodes root data directory")
	flag.StringVar(&ret.nodeName, "n", defaultNodeName, "Host node name")
	flag.BoolVar(&ret.runInCluster, "c", defaultRunInCluster, "True when running inside a k8 cluster")

	flag.Parse()

	ret.exitCodeInterrupt = exitCodeInterrupt

	return ret
}

func (c *Config) GetLogLevel() string {
	return c.logLevel
}

func (c *Config) GetExitCodeInterrupt() int {
	return c.exitCodeInterrupt
}

func (c *Config) GetDSAppName() string {
	return c.dsAppName
}

func (c *Config) GetDataDir() string {
	return c.dataDir
}

func (c *Config) GetTimerTick() int {
	return c.timerTick
}

func (c *Config) GetSelector() string {
	return c.selector
}

func (c *Config) ShouldRunInCluster() bool {
	return c.runInCluster
}

func (c *Config) GetNodeName() string {
	return c.nodeName
}
