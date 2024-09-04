// Copyright 2019 tsuru authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package config

import (
	"encoding/json"
	"flag"
	"log"
	"os"
	"reflect"
	"regexp"
	"strings"
	"sync"
	"time"

	"github.com/fsnotify/fsnotify"
	"github.com/mitchellh/mapstructure"
	"github.com/spf13/pflag"
	"github.com/spf13/viper"
	corev1 "k8s.io/api/core/v1"
	clientcmdapi "k8s.io/client-go/tools/clientcmd/api"
)

const (
	keyPrefix = "rpaasv2"
)

type RpaasConfig struct {
	WebSocketAllowedOrigins      []string                   `json:"websocket-allowed-origins"`
	Clusters                     []ClusterConfig            `json:"clusters"`
	ConfigDenyPatterns           []regexp.Regexp            `json:"config-deny-patterns"`
	ServiceName                  string                     `json:"service-name"`
	APIUsername                  string                     `json:"api-username"`
	APIPassword                  string                     `json:"api-password"`
	TLSCertificate               string                     `json:"tls-certificate"`
	TLSKey                       string                     `json:"tls-key"`
	DefaultAffinity              *corev1.Affinity           `json:"default-affinity"`
	TeamAffinity                 map[string]corev1.Affinity `json:"team-affinity"`
	SyncInterval                 time.Duration              `json:"sync-interval"`
	DashboardTemplate            string                     `json:"dashboard-template"`
	DefaultCertManagerIssuer     string                     `json:"default-cert-manager-issuer"`
	LoadBalancerNameLabelKey     string                     `json:"loadbalancer-name-label-key"`
	WebSocketHandshakeTimeout    time.Duration              `json:"websocket-handshake-timeout"`
	WebSocketReadBufferSize      int                        `json:"websocket-read-buffer-size"`
	WebSocketWriteBufferSize     int                        `json:"websocket-write-buffer-size"`
	WebSocketPingInterval        time.Duration              `json:"websocket-ping-interval"`
	WebSocketMaxIdleTime         time.Duration              `json:"websocket-max-idle-time"`
	WebSocketWriteWait           time.Duration              `json:"websocket-write-wait"`
	MultiCluster                 bool                       `json:"multi-cluster"`
	NamespacedInstances          bool                       `json:"namespaced-instances"`
	EnableCertManager            bool                       `json:"enable-cert-manager"`
	NewInstanceReplicas          int                        `json:"new-instance-replicas"`
	ForbiddenAnnotationsPrefixes []string                   `json:"forbidden-annotations-prefixes"`
	DebugImage                   string                     `json:"debug-image"`
}

type ClusterConfig struct {
	Name              string `json:"name"`
	Default           bool   `json:"default"`
	DisableValidation bool   `json:"disableValidation"`
	Address           string `json:"address"`
	Token             string `json:"token"`
	TokenFile         string `json:"tokenFile"`
	CA                string `json:"ca"`

	AuthProvider *clientcmdapi.AuthProviderConfig `json:"authProvider"`
	ExecProvider *clientcmdapi.ExecConfig         `json:"execProvider"`
}

var rpaasConfig struct {
	sync.RWMutex
	conf RpaasConfig
}

func Get() RpaasConfig {
	rpaasConfig.RLock()
	defer rpaasConfig.RUnlock()
	return rpaasConfig.conf
}

func Set(conf RpaasConfig) {
	rpaasConfig.Lock()
	defer rpaasConfig.Unlock()
	rpaasConfig.conf = conf
}

func Init() error {
	flagset := pflag.NewFlagSet(os.Args[0], pflag.ExitOnError)
	flagset.String("config", "", "RPaaS Config file")
	flagset.Bool("fake-api", false, "Run a fake API server, without K8s")
	pflag.CommandLine.AddFlagSet(flagset)
	pflag.CommandLine.AddGoFlagSet(flag.CommandLine)
	pflag.Parse()
	viper.BindPFlags(pflag.CommandLine)
	viper.SetEnvPrefix(keyPrefix)
	viper.SetEnvKeyReplacer(strings.NewReplacer(".", "_", "-", "_"))
	viper.BindEnv("api-username")
	viper.BindEnv("api-password")
	viper.BindEnv("tls-certificate")
	viper.BindEnv("tls-key")
	viper.SetDefault("service-name", keyPrefix)
	viper.SetDefault("tls-certificate", "")
	viper.SetDefault("tls-key", "")
	viper.SetDefault("sync-interval", 5*time.Minute)
	viper.SetDefault("websocket-handshake-timeout", 5*time.Second)
	viper.SetDefault("websocket-read-buffer-size", 1<<10)  // 1 KiB
	viper.SetDefault("websocket-write-buffer-size", 4<<10) // 4 KiB
	viper.SetDefault("websocket-ping-interval", 2*time.Second)
	viper.SetDefault("websocket-max-idle-time", 60*time.Second)
	viper.SetDefault("websocket-write-wait", time.Second)
	viper.SetDefault("enable-cert-manager", false)
	viper.SetDefault("new-instance-replicas", 1)
	viper.SetDefault("forbidden-annotations-prefixes", []string{"rpaas.extensions.tsuru.io", "afh.tsuru.io"})
	viper.SetDefault("debug-image", "")
	viper.AutomaticEnv()
	err := readConfig()
	if err != nil {
		return err
	}
	rpaasConfig.Lock()
	defer rpaasConfig.Unlock()
	var conf RpaasConfig
	err = unmarshalConfig(&conf)
	if err != nil {
		return err
	}
	rpaasConfig.conf = conf
	return nil
}

func unmarshalConfig(conf *RpaasConfig) error {
	decodeHook := mapstructure.ComposeDecodeHookFunc(
		mapstructure.StringToTimeDurationHookFunc(),
		mapstructure.StringToSliceHookFunc(","),
		jsonStringToMap,
		stringToRegexp,
	)
	err := viper.Unmarshal(conf, viper.DecodeHook(decodeHook), func(dc *mapstructure.DecoderConfig) {
		dc.TagName = "json"
	})
	return err
}

func readConfig() error {
	configPath := viper.GetString("config")
	if configPath == "" {
		return nil
	}
	log.Printf("Using config file from: %v", configPath)
	viper.SetConfigFile(configPath)
	err := viper.ReadInConfig()
	if err != nil {
		return err
	}
	viper.WatchConfig()
	viper.OnConfigChange(func(e fsnotify.Event) {
		rpaasConfig.Lock()
		defer rpaasConfig.Unlock()
		log.Printf("reloading config file: %v", e.Name)
		var conf RpaasConfig
		err = unmarshalConfig(&conf)
		if err != nil {
			log.Printf("error parsing new config file: %v", err)
		} else {
			rpaasConfig.conf = conf
		}
	})
	return nil
}

func stringToRegexp(src reflect.Type, target reflect.Type, data interface{}) (interface{}, error) {
	if src.Kind() != reflect.String || target != reflect.TypeOf(regexp.Regexp{}) {
		return data, nil
	}
	raw := data.(string)
	if raw == "" {
		return nil, nil
	}
	re, err := regexp.Compile(raw)
	if err != nil {
		return nil, err
	}
	return re, nil
}

func jsonStringToMap(f reflect.Kind, t reflect.Kind, data interface{}) (interface{}, error) {
	if f != reflect.String || t != reflect.Map {
		return data, nil
	}
	raw := data.(string)
	if raw == "" {
		return nil, nil
	}
	var ret map[string]string
	err := json.Unmarshal([]byte(raw), &ret)
	if err != nil {
		log.Printf("ignored error trying to parse %q as json: %v", raw, err)
		return data, nil
	}
	return ret, nil
}
