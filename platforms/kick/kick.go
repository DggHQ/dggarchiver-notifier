package kick

import (
	"encoding/json"
	"fmt"
	"io"
	"strings"
	"time"

	config "github.com/DggHQ/dggarchiver-config/notifier"
	log "github.com/DggHQ/dggarchiver-logger"
	dggarchivermodel "github.com/DggHQ/dggarchiver-model"
	"github.com/DggHQ/dggarchiver-notifier/state"
	"github.com/DggHQ/dggarchiver-notifier/util"
	http "github.com/bogdanfinn/fhttp"
	tls_client "github.com/bogdanfinn/tls-client"
	lua "github.com/yuin/gopher-lua"
	"golang.org/x/exp/slices"
)

type Platform struct {
	httpClient tls_client.HttpClient
	cfg        *config.Config
	state      *state.State
	prefix     string
	sleepTime  time.Duration
}

type api struct {
	URL        string `json:"playback_url"`
	Livestream struct {
		IsLive    bool   `json:"is_live"`
		ID        int    `json:"id"`
		Slug      string `json:"slug"`
		CreatedAt string `json:"created_at"`
		Title     string `json:"session_title"`
		Thumbnail struct {
			URL string `json:"responsive"`
		} `json:"thumbnail"`
	} `json:"livestream"`
}

// New returns a new Kick platform struct
func New(cfg *config.Config, state *state.State) *Platform {
	var err error

	p := Platform{
		cfg:       cfg,
		state:     state,
		prefix:    "[Kick] [SCRAPER]",
		sleepTime: time.Second * 60 * time.Duration(cfg.Notifier.Platforms.Kick.ScraperRefresh),
	}

	jar := tls_client.NewCookieJar()
	options := []tls_client.HttpClientOption{
		tls_client.WithTimeoutSeconds(30),
		tls_client.WithClientProfile(tls_client.Chrome_110),
		tls_client.WithNotFollowRedirects(),
		tls_client.WithCookieJar(jar),
	}

	if cfg.Notifier.Platforms.Kick.ProxyURL != "" {
		options = append(options, tls_client.WithProxyUrl(cfg.Notifier.Platforms.Kick.ProxyURL))
	}

	p.httpClient, err = tls_client.NewHttpClient(tls_client.NewNoopLogger(), options...)
	if err != nil {
		log.Fatalf("[Kick] [SCRAPER] Error while creating a TLS client: %s", err)
	}

	return &p
}

// GetPrefix returns a log prefix for platform p
func (p *Platform) GetPrefix() string {
	return p.prefix
}

// GetSleepTime returns sleep duration for platform p
func (p *Platform) GetSleepTime() time.Duration {
	return p.sleepTime
}

// CheckLivestream checks for an existing livestream on platform p,
// and, if found, publishes the info to NATS
func (p *Platform) CheckLivestream(l *lua.LState) error {
	stream := p.scrape()

	if stream != nil && stream.Livestream.IsLive {
		if !slices.Contains(p.state.SentVODs, fmt.Sprintf("kick:%d", stream.Livestream.ID)) {
			if p.state.CheckPriority("Kick", p.cfg) {
				log.Infof("[Kick] [SCRAPER] Found a currently running stream with ID %d", stream.Livestream.ID)
				if p.cfg.Notifier.Plugins.Enabled {
					util.LuaCallReceiveFunction(l, fmt.Sprintf("%d", stream.Livestream.ID))
				}

				vod := &dggarchivermodel.VOD{
					Platform:    "kick",
					Downloader:  p.cfg.Notifier.Platforms.Kick.Downloader,
					ID:          fmt.Sprintf("%d", stream.Livestream.ID),
					PlaybackURL: stream.URL,
					Title:       stream.Livestream.Title,
					StartTime:   time.Now().Format(time.RFC3339),
					EndTime:     "",
					Thumbnail:   strings.Split(strings.Split(stream.Livestream.Thumbnail.URL, ",")[0], " ")[0],
				}

				p.state.CurrentStreams.Kick = *vod

				bytes, err := json.Marshal(vod)
				if err != nil {
					log.Fatalf("[Kick] [SCRAPER] Couldn't marshal VOD with ID %s into a JSON object: %v", vod.ID, err)
				}

				if err = p.cfg.NATS.NatsConnection.Publish(fmt.Sprintf("%s.job", p.cfg.NATS.Topic), bytes); err != nil {
					log.Errorf("[Kick] [SCRAPER] Wasn't able to send message with VOD with ID %s: %v", vod.ID, err)
					return nil
				}

				if p.cfg.Notifier.Plugins.Enabled {
					util.LuaCallSendFunction(l, vod)
				}
				p.state.SentVODs = append(p.state.SentVODs, fmt.Sprintf("kick:%s", vod.ID))
				p.state.Dump()
			} else {
				log.Infof("[Kick] [SCRAPER] Stream with ID %d is being streamed on a different platform, skipping", stream.Livestream.ID)
			}
		} else {
			log.Infof("[Kick] [SCRAPER] Stream with ID %d was already sent", stream.Livestream.ID)
		}
	} else {
		p.state.CurrentStreams.Kick = dggarchivermodel.VOD{}
		log.Infof("[Kick] [SCRAPER] No stream found")
	}

	return nil
}

func (p *Platform) scrape() *api {
	req, err := http.NewRequest(http.MethodGet, fmt.Sprintf("https://kick.com/api/v1/channels/%s", p.cfg.Notifier.Platforms.Kick.Channel), nil)
	if err != nil {
		log.Fatalf("[Kick] [SCRAPER] Error creating a request: %s", err)
	}

	req.Header = http.Header{
		"accept":          {"text/html,application/xhtml+xml,application/xml;q=0.9,image/avif,image/webp,*/*;q=0.8"},
		"accept-language": {"en-US,en;q=0.5"},
		"user-agent":      {"Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/110.0.0.0 Safari/537.36"},
		http.HeaderOrderKey: {
			"accept",
			"accept-language",
			"user-agent",
		},
	}

	resp, err := p.httpClient.Do(req)
	if err != nil {
		log.Errorf("[Kick] [SCRAPER] Error making a request: %s", err)
		return nil
	}

	defer resp.Body.Close()
	bytes, err := io.ReadAll(resp.Body)
	if err != nil {
		log.Errorf("[Kick] [SCRAPER] Error reading the response: %s", err)
		return nil
	}

	var stream api
	err = json.Unmarshal(bytes, &stream)
	if err != nil {
		log.Errorf("[Kick] [SCRAPER] Error unmarshalling the response: %s", err)
		return nil
	}

	return &stream
}
