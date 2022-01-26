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

type ConfigOption func(c *Config)

func NewConfig(opts ...ConfigOption) *Config {
	ret := &Config{
		dsAppName:    defaultDSAppName,
		selector:     defaultSelector,
		logLevel:     defaultLogLevel,
		timerTick:    defaultTimerTick,
		dataDir:      defaultDataDir,
		nodeName:     defaultNodeName,
		runInCluster: defaultRunInCluster,
	}
	for _, opt := range opts {
		opt(ret)
	}

	return ret
}

func WithAppName(name string) ConfigOption {
	return func(c *Config) {
		c.dsAppName = name
	}
}

func WithSelector(name string) ConfigOption {
	return func(c *Config) {
		c.selector = name
	}
}

func WithLogLevel(level string) ConfigOption {
	return func(c *Config) {
		c.logLevel = level
	}
}

func WithTimerTick(t int) ConfigOption {
	return func(c *Config) {
		c.timerTick = t
	}
}

func WithDataDir(d string) ConfigOption {
	return func(c *Config) {
		c.dataDir = d
	}
}

func WithNodeName(n string) ConfigOption {
	return func(c *Config) {
		c.nodeName = n
	}
}

func WithRunInCluster(r bool) ConfigOption {
	return func(c *Config) {
		c.runInCluster = r
	}
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
