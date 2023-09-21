// Copyright 2023 Harness, Inc.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//	http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package server

import (
	"fmt"
	"os"
	"path/filepath"
	"unicode"

	"github.com/harness/gitness/events"
	"github.com/harness/gitness/gitrpc"
	"github.com/harness/gitness/gitrpc/server"
	"github.com/harness/gitness/internal/services/trigger"
	"github.com/harness/gitness/internal/services/webhook"
	"github.com/harness/gitness/lock"
	"github.com/harness/gitness/store/database"
	"github.com/harness/gitness/types"

	"github.com/kelseyhightower/envconfig"
	"golang.org/x/text/runes"
	"golang.org/x/text/transform"
	"golang.org/x/text/unicode/norm"
)

// LoadConfig returns the system configuration from the
// host environment.
func LoadConfig() (*types.Config, error) {
	config := new(types.Config)
	err := envconfig.Process("", config)
	if err != nil {
		return nil, err
	}

	config.InstanceID, err = getSanitizedMachineName()
	if err != nil {
		return nil, fmt.Errorf("unable to ensure that instance ID is set in config: %w", err)
	}

	return config, nil
}

// getSanitizedMachineName gets the name of the machine and returns it in sanitized format.
func getSanitizedMachineName() (string, error) {
	// use the hostname as default id of the instance
	hostName, err := os.Hostname()
	if err != nil {
		return "", err
	}

	// Always cast to lower and remove all unwanted chars
	// NOTE: this could theoretically lead to overlaps, then it should be passed explicitly
	// NOTE: for k8s names/ids below modifications are all noops
	// https://kubernetes.io/docs/concepts/overview/working-with-objects/names/

	// The following code will:
	// * remove invalid runes
	// * remove diacritical marks (ie "smörgåsbord" to "smorgasbord")
	// * lowercase A-Z to a-z
	// * leave only a-z, 0-9, '-', '.' and replace everything else with '_'
	hostName, _, err = transform.String(
		transform.Chain(
			norm.NFD,
			runes.ReplaceIllFormed(),
			runes.Remove(runes.In(unicode.Mn)),
			runes.Map(func(r rune) rune {
				switch {
				case 'A' <= r && r <= 'Z':
					return r + 32
				case 'a' <= r && r <= 'z':
					return r
				case '0' <= r && r <= '9':
					return r
				case r == '-', r == '.':
					return r
				default:
					return '_'
				}
			}),
			norm.NFC),
		hostName)
	if err != nil {
		return "", err
	}

	return hostName, nil
}

// ProvideDatabaseConfig loads the database config from the main config.
func ProvideDatabaseConfig(config *types.Config) database.Config {
	return database.Config{
		Driver:     config.Database.Driver,
		Datasource: config.Database.Datasource,
	}
}

// ProvideGitRPCServerConfig loads the gitrpc server config from the environment.
// It backfills certain config elements to work with cmdone.
func ProvideGitRPCServerConfig() (server.Config, error) {
	config := server.Config{}
	err := envconfig.Process("", &config)
	if err != nil {
		return server.Config{}, fmt.Errorf("failed to load gitrpc server config: %w", err)
	}
	if config.GitHookPath == "" {
		var executablePath string
		executablePath, err = os.Executable()
		if err != nil {
			return server.Config{}, fmt.Errorf("failed to get path of current executable: %w", err)
		}

		config.GitHookPath = executablePath
	}
	if config.GitRoot == "" {
		var homedir string
		homedir, err = os.UserHomeDir()
		if err != nil {
			return server.Config{}, err
		}

		config.GitRoot = filepath.Join(homedir, ".gitrpc")
	}

	return config, nil
}

// ProvideGitRPCClientConfig loads the gitrpc client config from the environment.
func ProvideGitRPCClientConfig() (gitrpc.Config, error) {
	config := gitrpc.Config{}
	err := envconfig.Process("", &config)
	if err != nil {
		return gitrpc.Config{}, fmt.Errorf("failed to load gitrpc client config: %w", err)
	}

	return config, nil
}

// ProvideEventsConfig loads the events config from the environment.
func ProvideEventsConfig() (events.Config, error) {
	config := events.Config{}
	err := envconfig.Process("", &config)
	if err != nil {
		return events.Config{}, fmt.Errorf("failed to load events config: %w", err)
	}

	return config, nil
}

// ProvideWebhookConfig loads the webhook service config from the main config.
func ProvideWebhookConfig(config *types.Config) webhook.Config {
	return webhook.Config{
		UserAgentIdentity:   config.Webhook.UserAgentIdentity,
		HeaderIdentity:      config.Webhook.HeaderIdentity,
		EventReaderName:     config.InstanceID,
		Concurrency:         config.Webhook.Concurrency,
		MaxRetries:          config.Webhook.MaxRetries,
		AllowPrivateNetwork: config.Webhook.AllowPrivateNetwork,
		AllowLoopback:       config.Webhook.AllowLoopback,
	}
}

// ProvideTriggerConfig loads the trigger service config from the main config.
func ProvideTriggerConfig(config *types.Config) trigger.Config {
	return trigger.Config{
		EventReaderName: config.InstanceID,
		Concurrency:     config.Webhook.Concurrency,
		MaxRetries:      config.Webhook.MaxRetries,
	}
}

// ProvideLockConfig generates the `lock` package config from the gitness config.
func ProvideLockConfig(config *types.Config) lock.Config {
	return lock.Config{
		App:           config.Lock.AppNamespace,
		Namespace:     config.Lock.DefaultNamespace,
		Provider:      lock.Provider(config.Lock.Provider),
		Expiry:        config.Lock.Expiry,
		Tries:         config.Lock.Tries,
		RetryDelay:    config.Lock.RetryDelay,
		DriftFactor:   config.Lock.DriftFactor,
		TimeoutFactor: config.Lock.TimeoutFactor,
	}
}
