package config

import (
	"context"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	log "github.com/DggHQ/dggarchiver-logger"
	"github.com/joho/godotenv"
	"github.com/nats-io/nats.go"
	"golang.org/x/oauth2/google"
	"google.golang.org/api/option"
	"google.golang.org/api/youtube/v3"
)

type Flags struct {
	Verbose bool
}

type YTConfig struct {
	YTChannel     string
	YTHealthCheck string
	YTRefresh     int
	YTAPIRefresh  int
	GoogleCred    string
	Service       *youtube.Service
}

type NATSConfig struct {
	Host           string
	Topic          string
	NatsConnection *nats.Conn
}

type PluginConfig struct {
	On           bool
	PathToScript string
}

type Config struct {
	Flags        Flags
	YTConfig     YTConfig
	NATSConfig   NATSConfig
	PluginConfig PluginConfig
}

func (cfg *Config) loadDotEnv() {
	var err error

	log.Debugf("Loading environment variables")
	godotenv.Load()

	// Flags
	verbose := strings.ToLower(os.Getenv("VERBOSE"))
	if verbose == "1" || verbose == "true" {
		cfg.Flags.Verbose = true
	}

	// YouTube
	cfg.YTConfig.GoogleCred = os.Getenv("GOOGLE_CRED")
	if cfg.YTConfig.GoogleCred == "" {
		log.Fatalf("Please set the GOOGLE_CRED environment variable and restart the app")
	}
	cfg.YTConfig.YTChannel = os.Getenv("YT_CHANNEL")
	if cfg.YTConfig.YTChannel == "" {
		log.Fatalf("Please set the YT_CHANNEL environment variable and restart the app")
	}
	cfg.YTConfig.YTHealthCheck = os.Getenv("YT_HEALTHCHECK")
	ytrefreshStr := os.Getenv("YT_REFRESH")
	if ytrefreshStr == "" {
		ytrefreshStr = "0"
	}
	cfg.YTConfig.YTRefresh, err = strconv.Atoi(ytrefreshStr)
	if err != nil {
		log.Fatalf("strconv error: %s", err)
	}
	ytapirefreshStr := os.Getenv("YT_API_REFRESH")
	if ytapirefreshStr == "" {
		ytapirefreshStr = "0"
	}
	cfg.YTConfig.YTAPIRefresh, err = strconv.Atoi(ytapirefreshStr)
	if err != nil {
		log.Fatalf("strconv error: %s", err)
	}

	// NATS Host Name or IP
	cfg.NATSConfig.Host = os.Getenv("NATS_HOST")
	if cfg.NATSConfig.Host == "" {
		log.Fatalf("Please set the NATS_HOST environment variable and restart the app")
	}

	// NATS Topic Name
	cfg.NATSConfig.Topic = os.Getenv("NATS_TOPIC")
	if cfg.NATSConfig.Topic == "" {
		log.Fatalf("Please set the NATS_TOPIC environment variable and restart the app")
	}

	// Lua Plugins
	plugins := strings.ToLower(os.Getenv("PLUGINS"))
	if plugins == "1" || plugins == "true" {
		cfg.PluginConfig.On = true
		cfg.PluginConfig.PathToScript = os.Getenv("LUA_PATH_TO_SCRIPT")
		if cfg.PluginConfig.PathToScript == "" {
			log.Fatalf("Please set the LUA_PATH_TO_SCRIPT environment variable and restart the app")
		}
	}

	log.Debugf("Environment variables loaded successfully")
}

func (cfg *Config) createGoogleClients() {
	log.Debugf("Creating Google API clients")

	ctx := context.Background()

	credpath := filepath.Join(".", cfg.YTConfig.GoogleCred)
	b, err := os.ReadFile(credpath)
	if err != nil {
		log.Fatalf("Unable to read client secret file: %v", err)
	}

	googleCfg, err := google.JWTConfigFromJSON(b, "https://www.googleapis.com/auth/youtube.readonly")
	if err != nil {
		log.Fatalf("Unable to parse client secret file to config: %v", err)
	}
	client := googleCfg.Client(ctx)

	cfg.YTConfig.Service, err = youtube.NewService(ctx, option.WithHTTPClient(client))
	if err != nil {
		log.Fatalf("Unable to retrieve YouTube client: %v", err)
	}

	log.Debugf("Created Google API clients successfully")
}

func (cfg *Config) loadNats() {
	// Connect to NATS server
	nc, err := nats.Connect(cfg.NATSConfig.Host, nil, nats.PingInterval(20*time.Second), nats.MaxPingsOutstanding(5))
	if err != nil {
		log.Fatalf("Could not connect to NATS server: %s", err)
	}
	log.Infof("Successfully connected to NATS server: %s", cfg.NATSConfig.Host)
	cfg.NATSConfig.NatsConnection = nc
}

func (cfg *Config) Initialize() {
	cfg.loadDotEnv()
	cfg.createGoogleClients()
	cfg.loadNats()
}
