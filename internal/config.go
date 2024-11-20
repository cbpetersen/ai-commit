package internal

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"

	"github.com/BurntSushi/toml"
	"github.com/charmbracelet/huh"
	"github.com/charmbracelet/log"
)

type Config struct {
	Azure struct {
		Key string `toml:"key"`
		URL string `toml:"url"`
	} `toml:"settings"`
}

func GetConfig(showConfig bool) (*Config, error) {
	homeDir, err := os.UserHomeDir()

	if err != nil {
		log.Errorf("Failed to get home directory: %v", err)
		return nil, err
	}
	configPath := filepath.Join(homeDir, ".config", "ai-commit", "ai-commit.toml")
	// log.Debugf("Config path: %s", configPath)
	config := Config{}
	if _, err := os.Stat(configPath); os.IsNotExist(err) || showConfig {
		configForm := huh.NewForm(
			huh.NewGroup(
				huh.NewInput().
					Title("What’s your Azure Key?").
					Value(&config.Azure.Key).
					EchoMode(huh.EchoModePassword).
					Validate(func(str string) error {
						if str == "" {
							return errors.New("Sorry, this cannot be empty")
						}
						return nil
					}),
				huh.NewInput().
					Title("What’s your Azure URL?").
					Value(&config.Azure.URL).
					Validate(func(str string) error {
						if str == "" {
							return errors.New("Sorry, this cannot be empty")
						}
						return nil
					}),
			),
		)
		configForm.Run()
		if err = os.MkdirAll(filepath.Dir(configPath), os.ModePerm); err != nil {
			fmt.Println("Failed to create config directory:", err)
			return nil, err
		}
		// Save the config as a TOML file
		f, err := os.Create(configPath)
		if err != nil {
			log.Errorf("Failed to create config file: %v", err)
			return nil, err
		}
		defer f.Close()

		if err := toml.NewEncoder(f).Encode(config); err != nil {
			return nil, errors.New("Failed to reaand encode config")
		}
		log.Debug("Config saved")
	} else {
		if _, err := toml.DecodeFile(configPath, &config); err != nil {
			fmt.Println("Failed to load config:", err)
			return nil, err
		}
		log.Debug("Config loaded")
		if config.Azure.Key == "" || config.Azure.URL == "" {
			return nil, errors.New("Azure Key and URL must be set in the config, run with --config to update the config")
		}
	}
	return &config, nil
}
