package configure

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"strings"

	"github.com/fsnotify/fsnotify"
	"github.com/kr/pretty"
	"github.com/sirupsen/logrus"
	"github.com/spf13/pflag"
	"github.com/spf13/viper"
)

type ServerCfg struct {
	Level      string `mapstructure:"level" json:"level"`
	ConfigFile string `mapstructure:"config_file" json:"config_file"`

	RedisURI string `mapstructure:"redis_uri" json:"redis_uri"`

	ConnURI  string `mapstructure:"conn_uri" json:"conn_uri"`
	ConnType string `mapstructure:"conn_type" json:"conn_type"`

	NodeID string `mapstructure:"node_id" json:"node_id"`

	ExitCode int `mapstructure:"exit_code" json:"exit_code"`
}

// default config
var defaultConf = ServerCfg{
	ConfigFile: "config.yaml",
}

var Config = viper.New()

func initLog() {
	if l, err := logrus.ParseLevel(Config.GetString("level")); err == nil {
		logrus.SetLevel(l)
		logrus.SetReportCaller(true)
	}
}

func checkErr(err error) {
	if err != nil {
		logrus.WithError(err).Fatal("config")
	}
}

// Capture environment variables
var NodeName string = os.Getenv("NODE_NAME")
var PodName string = os.Getenv("POD_NAME")
var PodIP string = os.Getenv("POD_IP")

func init() {
	logrus.SetFormatter(&logrus.JSONFormatter{})
	// Default config
	b, _ := json.Marshal(defaultConf)
	defaultConfig := bytes.NewReader(b)
	viper.SetConfigType("json")
	checkErr(viper.ReadConfig(defaultConfig))
	checkErr(Config.MergeConfigMap(viper.AllSettings()))

	// Flags
	pflag.String("config_file", "config.yaml", "configure filename")
	pflag.String("level", "info", "Log level")
	pflag.String("redis_uri", "", "Address for the redis server.")

	pflag.String("conn_uri", "", "Connection url:port or path")
	pflag.String("conn_type", "", "Connection type, udp/tcp/unix")

	pflag.String("node_id", "", "Used in the response header of a requset X-Node-ID")
	pflag.String("node_name", "", "Used in the response header of a requset X-Node-Name")

	pflag.Bool("disable_redis_cache", false, "Disable the redis cache for mongodb")

	pflag.String("version", "1.0", "Version of the system.")
	pflag.Int("exit_code", 0, "Status code for successful and graceful shutdown, [0-125].")
	pflag.Parse()
	checkErr(Config.BindPFlags(pflag.CommandLine))

	// File
	Config.SetConfigFile(Config.GetString("config_file"))
	Config.AddConfigPath(".")
	err := Config.ReadInConfig()
	if err != nil {
		logrus.Warning(err)
		logrus.Info("Using default config")
	} else {
		checkErr(Config.MergeInConfig())
	}

	// Environment
	replacer := strings.NewReplacer(".", "_")
	Config.SetEnvKeyReplacer(replacer)
	Config.AllowEmptyEnv(true)
	Config.AutomaticEnv()

	// Log
	initLog()

	// Print final config
	c := ServerCfg{}
	checkErr(Config.Unmarshal(&c))
	logrus.Debugf("Current configurations: \n%# v", pretty.Formatter(c))

	Config.WatchConfig()
	Config.OnConfigChange(func(_ fsnotify.Event) {
		fmt.Println("Config has changed")
	})
}
