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

	"github.com/fsnotify/fsnotify"
	"github.com/mitchellh/mapstructure"
	"github.com/spf13/pflag"
	"github.com/spf13/viper"
	"github.com/tsuru/rpaas-operator/pkg/apis/extensions/v1alpha1"
	corev1 "k8s.io/api/core/v1"
)

const (
	keyPrefix = "rpaasv2"
)

type RpaasConfig struct {
	ServiceName        string                     `json:"service-name"`
	ServiceAnnotations map[string]string          `json:"service-annotations"`
	APIUsername        string                     `json:"api-username"`
	APIPassword        string                     `json:"api-password"`
	TLSCertificate     string                     `json:"tls-certificate"`
	TLSKey             string                     `json:"tls-key"`
	DefaultAffinity    *corev1.Affinity           `json:"default-affinity"`
	TeamAffinity       map[string]corev1.Affinity `json:"team-affinity"`

	Flavors []FlavorConfig
}

type FlavorConfig struct {
	Name        string
	Description string
	Spec        v1alpha1.RpaasPlanSpec
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
	viper.BindEnv("service-annotations")
	viper.BindEnv("tls-certificate")
	viper.BindEnv("tls-key")
	viper.SetDefault("service-name", keyPrefix)
	viper.SetDefault("tls-certificate", "")
	viper.SetDefault("tls-key", "")
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
