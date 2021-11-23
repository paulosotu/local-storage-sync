package models

import (
	"flag"
)

const (
	exitCodeErr         = 1
	exitCodeInterrupt   = 0
	defaultLogLevel     = "Info"
	defaultTimerTick    = 10
	defaultDataDir      = "/data-sync"
	defaultNodeName     = "my-node"
	defaultRunInCluster = false
	defaultNamespace    = "default"
	defaultSelector     = ""
)

type Config struct {
	ExitCodeErr       int
	ExitCodeInterrupt int
	TimerTick         int
	DataDir           string
	NodeName          string
	LogLevel          string
	Namespace         string
	Selector          string
	RunInCluster      bool
}

func NewConfigFromArgs() *Config {
	ret := &Config{}
	// do a piece of work,
	flag.StringVar(&ret.Namespace, "namespace", defaultNamespace, "k8 Namespace where the stateful set are running")
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
