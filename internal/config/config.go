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
	"strings"
	"sync"
	"time"

	"github.com/fsnotify/fsnotify"
	"github.com/mitchellh/mapstructure"
	"github.com/spf13/pflag"
	"github.com/spf13/viper"
	corev1 "k8s.io/api/core/v1"
)

const (
	keyPrefix = "rpaasv2"

	DefaultPortRangeMin = 20000
	DefaultPortRangeMax = 30000
)

type RpaasConfig struct {
	ServiceName                          string                     `json:"service-name"`
	APIUsername                          string                     `json:"api-username"`
	APIPassword                          string                     `json:"api-password"`
	TLSCertificate                       string                     `json:"tls-certificate"`
	TLSKey                               string                     `json:"tls-key"`
	DefaultAffinity                      *corev1.Affinity           `json:"default-affinity"`
	TeamAffinity                         map[string]corev1.Affinity `json:"team-affinity"`
	SyncInterval                         time.Duration              `json:"sync-interval"`
	PortRangeMin                         int32                      `json:"port-range-min"`
	PortRangeMax                         int32                      `json:"port-range-max"`
	LoadBalancerNameLabelKey             string                     `json:"loadbalancer-name-label-key"`
	WebSocketHandshakeTimeout            time.Duration              `json:"websocket-handshake-timeout"`
	WebSocketReadBufferSize              int                        `json:"websocket-read-buffer-size"`
	WebSocketWriteBufferSize             int                        `json:"websocket-write-buffer-size"`
	WebSocketPingInterval                time.Duration              `json:"websocket-ping-interval"`
	WebSocketMaxIdleTime                 time.Duration              `json:"websocket-max-idle-time"`
	WebSocketWriteWait                   time.Duration              `json:"websocket-write-wait"`
	WebSocketAllowedOrigins              []string                   `json:"websocket-allowed-origins"`
	SuppressPrivateKeyOnCertificatesList bool                       `json:"suppress-private-key-on-certificates-list"`
	MultiCluster                         bool                       `json:"multi-cluster"`
	Clusters                             []ClusterConfig            `json:"clusters"`
}

type ClusterConfig struct {
	Name      string `json:"name"`
	Default   bool   `json:"default"`
	Address   string `json:"address"`
	Token     string `json:"token"`
	TokenFile string `json:"tokenFile"`
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
	viper.SetDefault("port-range-min", DefaultPortRangeMin)
	viper.SetDefault("port-range-max", DefaultPortRangeMax)
	viper.SetDefault("websocket-handshake-timeout", 5*time.Second)
	viper.SetDefault("websocket-read-buffer-size", 1<<10)  // 1 KiB
	viper.SetDefault("websocket-write-buffer-size", 4<<10) // 4 KiB
	viper.SetDefault("websocket-ping-interval", 2*time.Second)
	viper.SetDefault("websocket-max-idle-time", 60*time.Second)
	viper.SetDefault("websocket-write-wait", time.Second)
	viper.AutomaticEnv()
	err := readConfig()
	if err != nil {
		return err
	}
	rpaasConfig.Lock()
	defer rpaasConfig.Unlock()
	var conf RpaasConfig
	decodeHook := mapstructure.ComposeDecodeHookFunc(
		mapstructure.StringToTimeDurationHookFunc(),
		mapstructure.StringToSliceHookFunc(","),
		jsonStringToMap,
	)
	err = viper.Unmarshal(&conf, viper.DecodeHook(decodeHook), func(dc *mapstructure.DecoderConfig) {
		dc.TagName = "json"
	})
	if err != nil {
		return err
	}
	rpaasConfig.conf = conf
	return nil
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
		err = viper.Unmarshal(&conf)
		if err != nil {
			log.Printf("error parsing new config file: %v", err)
		} else {
			rpaasConfig.conf = conf
		}
	})
	return nil
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
