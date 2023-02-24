package config

import (
	"context"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	log "github.com/DggHQ/dggarchiver-logger"
	"github.com/joho/godotenv"
	amqp "github.com/rabbitmq/amqp091-go"
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

type AMQPConfig struct {
	URI          string
	ExchangeName string
	ExchangeType string
	QueueName    string
	Context      context.Context
	Channel      *amqp.Channel
	connection   *amqp.Connection
}

type PluginConfig struct {
	On           bool
	PathToScript string
}

type Config struct {
	Flags        Flags
	YTConfig     YTConfig
	AMQPConfig   AMQPConfig
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

	// AMQP
	cfg.AMQPConfig.URI = os.Getenv("AMQP_URI")
	if cfg.AMQPConfig.URI == "" {
		log.Fatalf("Please set the AMQP_URI environment variable and restart the app")
	}
	cfg.AMQPConfig.ExchangeName = ""
	cfg.AMQPConfig.ExchangeType = "direct"
	cfg.AMQPConfig.QueueName = "notifier"

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

func (cfg *Config) loadAMQP() {
	var err error

	cfg.AMQPConfig.Context = context.Background()

	cfg.AMQPConfig.connection, err = amqp.Dial(cfg.AMQPConfig.URI)
	if err != nil {
		log.Fatalf("Wasn't able to connect to the AMQP server: %s", err)
	}

	cfg.AMQPConfig.Channel, err = cfg.AMQPConfig.connection.Channel()
	if err != nil {
		log.Fatalf("Wasn't able to create the AMQP channel: %s", err)
	}

	_, err = cfg.AMQPConfig.Channel.QueueDeclare(
		cfg.AMQPConfig.QueueName, // queue name
		true,                     // durable
		false,                    // auto delete
		false,                    // exclusive
		false,                    // no wait
		nil,                      // arguments
	)
	if err != nil {
		log.Fatalf("Wasn't able to declare the AMQP queue: %s", err)
	}
}

func (cfg *Config) Initialize() {
	cfg.loadDotEnv()
	cfg.createGoogleClients()
	cfg.loadAMQP()
}
