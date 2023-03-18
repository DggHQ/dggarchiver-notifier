package config

import (
	"context"
	"os"
	"path/filepath"
	"reflect"
	"time"

	log "github.com/DggHQ/dggarchiver-logger"
	"github.com/joho/godotenv"
	"github.com/nats-io/nats.go"
	"golang.org/x/oauth2/google"
	"google.golang.org/api/option"
	"google.golang.org/api/youtube/v3"
	"gopkg.in/yaml.v3"
)

type Flags struct {
	Verbose bool
}

type KickConfig struct {
	Enabled        bool
	Channel        string `yaml:"channel"`
	HealthCheck    string `yaml:"healthcheck"`
	ScraperRefresh int    `yaml:"scraper_refresh"`
}

type RumbleConfig struct {
	Enabled        bool
	Channel        string `yaml:"channel"`
	HealthCheck    string `yaml:"healthcheck"`
	ScraperRefresh int    `yaml:"scraper_refresh"`
}

type YTConfig struct {
	Enabled        bool
	Channel        string `yaml:"channel"`
	HealthCheck    string `yaml:"healthcheck"`
	ScraperRefresh int    `yaml:"scraper_refresh"`
	APIRefresh     int    `yaml:"api_refresh"`
	GoogleCred     string `yaml:"google_credentials"`
	Service        *youtube.Service
}

type NATSConfig struct {
	Host           string `yaml:"host"`
	Topic          string `yaml:"topic"`
	NatsConnection *nats.Conn
}

type PluginConfig struct {
	Enabled      bool   `yaml:"enabled"`
	PathToPlugin string `yaml:"path"`
}

type Config struct {
	Notifier struct {
		Verbose   bool
		Platforms struct {
			YTConfig     YTConfig     `yaml:"youtube"`
			RumbleConfig RumbleConfig `yaml:"rumble"`
			KickConfig   KickConfig   `yaml:"kick"`
		}
		PluginConfig PluginConfig `yaml:"plugins"`
		NATSConfig   NATSConfig   `yaml:"nats"`
	}
}

func (cfg *Config) checkPlatforms() bool {
	var enabledPlatforms int
	platformsValue := reflect.ValueOf(cfg.Notifier.Platforms)
	platformsFields := reflect.VisibleFields(reflect.TypeOf(cfg.Notifier.Platforms))
	for _, field := range platformsFields {
		if platformsValue.FieldByName(field.Name).FieldByName("Enabled").Bool() {
			enabledPlatforms++
		}
	}
	return enabledPlatforms > 0
}

func (cfg *Config) loadConfig() {
	var err error

	log.Debugf("Loading the service configuration")
	godotenv.Load()

	configFile := os.Getenv("CONFIG")
	if configFile == "" {
		configFile = "config.yaml"
	}
	configBytes, err := os.ReadFile(configFile)
	if err != nil {
		log.Fatalf("Config load error: %s", err)
	}

	err = yaml.Unmarshal(configBytes, &cfg)
	if err != nil {
		log.Fatalf("YAML unmarshalling error: %s", err)
	}

	if !cfg.checkPlatforms() {
		log.Fatalf("Please enable at least one platform and restart the service")
	}

	// YouTube
	if cfg.Notifier.Platforms.YTConfig.Enabled {
		if cfg.Notifier.Platforms.YTConfig.GoogleCred == "" {
			log.Fatalf("Please set the youtube:google_credentials config variable and restart the service")
		}
		if cfg.Notifier.Platforms.YTConfig.Channel == "" {
			log.Fatalf("Please set the youtube:channel config variable and restart the service")
		}
		if cfg.Notifier.Platforms.YTConfig.ScraperRefresh == 0 && cfg.Notifier.Platforms.YTConfig.APIRefresh == 0 {
			log.Fatalf("Please set the youtube:scraper_refresh or the youtube:api_refresh config variable and restart the service")
		}
		cfg.createGoogleClients()
	}

	// Rumble
	if cfg.Notifier.Platforms.RumbleConfig.Enabled {
		if cfg.Notifier.Platforms.RumbleConfig.Channel == "" {
			log.Fatalf("Please set the rumble:channel config variable and restart the service")
		}
		if cfg.Notifier.Platforms.RumbleConfig.ScraperRefresh == 0 {
			log.Fatalf("Please set the rumble:scraper_refresh config variable and restart the service")
		}
	}

	// Kick
	if cfg.Notifier.Platforms.KickConfig.Enabled {
		if cfg.Notifier.Platforms.KickConfig.Channel == "" {
			log.Fatalf("Please set the kick:channel config variable and restart the service")
		}
		if cfg.Notifier.Platforms.KickConfig.ScraperRefresh == 0 {
			log.Fatalf("Please set the kick:scraper_refresh config variable and restart the service")
		}
	}

	// NATS Host Name or IP
	if cfg.Notifier.NATSConfig.Host == "" {
		log.Fatalf("Please set the nats:host config variable and restart the service")
	}

	// NATS Topic Name
	if cfg.Notifier.NATSConfig.Topic == "" {
		log.Fatalf("Please set the nats:topic config variable and restart the service")
	}

	// Lua Plugins
	if cfg.Notifier.PluginConfig.Enabled {
		if cfg.Notifier.PluginConfig.PathToPlugin == "" {
			log.Fatalf("Please set the plugins:path config variable and restart the service")
		}
	}

	log.Debugf("Config loaded successfully")
}

func (cfg *Config) createGoogleClients() {
	log.Debugf("Creating Google API clients")

	ctx := context.Background()

	credpath := filepath.Join(".", cfg.Notifier.Platforms.YTConfig.GoogleCred)
	b, err := os.ReadFile(credpath)
	if err != nil {
		log.Fatalf("Unable to read client secret file: %v", err)
	}

	googleCfg, err := google.JWTConfigFromJSON(b, "https://www.googleapis.com/auth/youtube.readonly")
	if err != nil {
		log.Fatalf("Unable to parse client secret file to config: %v", err)
	}
	client := googleCfg.Client(ctx)

	cfg.Notifier.Platforms.YTConfig.Service, err = youtube.NewService(ctx, option.WithHTTPClient(client))
	if err != nil {
		log.Fatalf("Unable to retrieve YouTube client: %v", err)
	}

	log.Debugf("Created Google API clients successfully")
}

func (cfg *Config) loadNats() {
	// Connect to NATS server
	nc, err := nats.Connect(cfg.Notifier.NATSConfig.Host, nil, nats.PingInterval(20*time.Second), nats.MaxPingsOutstanding(5))
	if err != nil {
		log.Fatalf("Could not connect to NATS server: %s", err)
	}
	log.Infof("Successfully connected to NATS server: %s", cfg.Notifier.NATSConfig.Host)
	cfg.Notifier.NATSConfig.NatsConnection = nc
}

func (cfg *Config) Initialize() {
	cfg.loadConfig()
	cfg.loadNats()
}
