package configure

import (
	"bytes"
	"encoding/json"
	"reflect"
	"strings"

	"github.com/sirupsen/logrus"
	"github.com/spf13/pflag"
	"github.com/spf13/viper"
)

type Config struct {
	Level      string `mapstructure:"level" json:"level"`
	ConfigFile string `mapstructure:"config_file" json:"config_file"`
	NoHeader   bool   `mapstructure:"noheader" json:"noheader"`

	Redis struct {
		Username   string   `mapstructure:"username" json:"username"`
		Password   string   `mapstructure:"password" json:"password"`
		Addresses  []string `mapstructure:"addresses" json:"addresses"`
		Database   int      `mapstructure:"database" json:"database"`
		Sentinel   bool     `mapstructure:"sentinel" json:"sentinel"`
		MasterName string   `mapstructure:"master_name" json:"master_name"`
	} `mapstructure:"redis" json:"redis"`

	API struct {
		Enabled           bool   `mapstructure:"enabled" json:"enabled"`
		Bind              string `mapstructure:"bind" json:"bind"`
		HeartbeatInterval int64  `mapstructure:"heartbeat_interval" json:"heartbeat_interval"`
	} `mapstructure:"api" json:"api"`

	Monitoring struct {
		Enabled bool       `mapstructure:"enabled" json:"enabled"`
		Bind    string     `mapstructure:"bind" json:"bind"`
		Labels  []KeyValue `mapstructure:"labels" json:"labels"`
	} `mapstructure:"monitoring" json:"monitoring"`

	Health struct {
		Enabled bool   `mapstructure:"enabled" json:"enabled"`
		Bind    string `mapstructure:"bind" json:"bind"`
	} `mapstructure:"health" json:"health"`

	Pod struct {
		Name string `mapstructure:"name" json:"name"`
	} `mapstructure:"pod" json:"pod"`
}

type KeyValue struct {
	Key   string `mapstructure:"key" json:"key"`
	Value string `mapstructure:"value" json:"value"`
}

func checkErr(err error) {
	if err != nil {
		logrus.WithError(err).Fatal("config")
	}
}

func New() *Config {
	initLogging("fatal")

	config := viper.New()

	// Default config
	b, _ := json.Marshal(Config{
		ConfigFile: "config.yaml",
	})

	tmp := viper.New()
	defaultConfig := bytes.NewReader(b)
	tmp.SetConfigType("json")
	checkErr(tmp.ReadConfig(defaultConfig))
	checkErr(config.MergeConfigMap(viper.AllSettings()))

	// Flags
	pflag.String("config", "config.yaml", "Config file location")
	pflag.Bool("noheader", false, "Disable the startup header")
	pflag.Parse()
	checkErr(config.BindPFlags(pflag.CommandLine))

	// File
	config.SetConfigFile(config.GetString("config"))
	config.AddConfigPath(".")
	checkErr(config.ReadInConfig())
	checkErr(config.MergeInConfig())

	BindEnvs(config, Config{})

	// Environment
	config.AutomaticEnv()
	config.SetEnvPrefix("EVENTS")
	config.SetEnvKeyReplacer(strings.NewReplacer(".", "_"))
	config.AllowEmptyEnv(true)

	// Print final config
	c := &Config{}
	checkErr(config.Unmarshal(&c))

	initLogging(c.Level)

	return c
}

func BindEnvs(config *viper.Viper, iface interface{}, parts ...string) {
	ifv := reflect.ValueOf(iface)
	ift := reflect.TypeOf(iface)
	for i := 0; i < ift.NumField(); i++ {
		v := ifv.Field(i)
		t := ift.Field(i)
		tv, ok := t.Tag.Lookup("mapstructure")
		if !ok {
			continue
		}
		switch v.Kind() {
		case reflect.Struct:
			BindEnvs(config, v.Interface(), append(parts, tv)...)
		default:
			_ = config.BindEnv(strings.Join(append(parts, tv), "."))
		}
	}
}
