package yt

import (
	"encoding/json"
	"errors"
	"fmt"
	"regexp"
	"strings"
	"time"

	config "github.com/DggHQ/dggarchiver-config/notifier"
	log "github.com/DggHQ/dggarchiver-logger"
	dggarchivermodel "github.com/DggHQ/dggarchiver-model"
	"github.com/DggHQ/dggarchiver-notifier/state"
	"github.com/DggHQ/dggarchiver-notifier/util"
	"github.com/gocolly/colly/v2"
	lua "github.com/yuin/gopher-lua"
	"golang.org/x/exp/slices"
)

var ytRegexp = regexp.MustCompile(`\/watch\?v=([^\"]*)`)

type Scraper struct {
	c         *colly.Collector
	index     int
	idChan    chan string
	cfg       *config.Config
	state     *state.State
	prefix    string
	sleepTime time.Duration
}

// New returns a new YouTube Scraper platform struct
func NewScraper(cfg *config.Config, state *state.State) *Scraper {
	c := colly.NewCollector()
	// disable cookie handling to bypass youtube consent screen
	c.DisableCookies()
	c.AllowURLRevisit = true

	idChan := make(chan string)

	p := Scraper{
		c:         c,
		idChan:    idChan,
		cfg:       cfg,
		state:     state,
		prefix:    "[YT] [SCRAPER]",
		sleepTime: time.Second * 60 * time.Duration(cfg.Notifier.Platforms.YouTube.ScraperRefresh),
	}

	c.OnResponse(func(r *colly.Response) {
		p.index = strings.Index(string(r.Body), "Started streaming ")
	})

	c.OnHTML("link[href][rel='canonical']", func(h *colly.HTMLElement) {
		go func() {
			if p.index != -1 {
				idChan <- ytRegexp.FindStringSubmatch(h.Attr("href"))[1]
			} else {
				idChan <- ""
			}
		}()
	})

	return &p
}

// GetPrefix returns a log prefix for platform p
func (p *Scraper) GetPrefix() string {
	return p.prefix
}

// GetSleepTime returns sleep duration for platform p
func (p *Scraper) GetSleepTime() time.Duration {
	return p.sleepTime
}

// CheckLivestream checks for an existing livestream on platform p,
// and, if found, publishes the info to NATS
func (p *Scraper) CheckLivestream(l *lua.LState) error {
	id := p.scrape()

	if id != "" {
		if !slices.Contains(p.state.SentVODs, fmt.Sprintf("youtube:%s", id)) {
			if p.state.CheckPriority("YouTube", p.cfg) {
				log.Infof("[YT] [SCRAPER] Found a currently running stream with ID %s", id)
				if p.cfg.Notifier.Plugins.Enabled {
					util.LuaCallReceiveFunction(l, id)
				}
				vid, _, err := getVideoInfo(p.cfg, id, "")
				if err != nil && !errors.Is(err, errIsNotModified) {
					return err
				}

				vod := &dggarchivermodel.VOD{
					Platform:   "youtube",
					Downloader: p.cfg.Notifier.Platforms.YouTube.Downloader,
					ID:         vid[0].Id,
					PubTime:    vid[0].Snippet.PublishedAt,
					Title:      vid[0].Snippet.Title,
					StartTime:  vid[0].LiveStreamingDetails.ActualStartTime,
					EndTime:    vid[0].LiveStreamingDetails.ActualEndTime,
					Thumbnail:  vid[0].Snippet.Thumbnails.Medium.Url,
				}

				p.state.CurrentStreams.YouTube = *vod

				bytes, err := json.Marshal(vod)
				if err != nil {
					log.Fatalf("[YT] [SCRAPER] Couldn't marshal VOD with ID %s into a JSON object: %v", vod.ID, err)
				}

				if err = p.cfg.NATS.NatsConnection.Publish(fmt.Sprintf("%s.job", p.cfg.NATS.Topic), bytes); err != nil {
					log.Errorf("[YT] [SCRAPER] Wasn't able to send message with VOD with ID %s: %v", vod.ID, err)
					return nil
				}

				if p.cfg.Notifier.Plugins.Enabled {
					util.LuaCallSendFunction(l, vod)
				}
				p.state.SentVODs = append(p.state.SentVODs, fmt.Sprintf("youtube:%s", vod.ID))
				p.state.Dump()
			} else {
				log.Infof("[YT] [SCRAPER] Stream with ID %s is being streamed on a different platform, skipping", id)
			}
		} else {
			log.Infof("[YT] [SCRAPER] Stream with ID %s was already sent", id)
		}
	} else {
		p.state.CurrentStreams.YouTube = dggarchivermodel.VOD{}
		log.Infof("[YT] [SCRAPER] No stream found")
	}

	util.HealthCheck(p.cfg.Notifier.Platforms.YouTube.HealthCheck)

	return nil
}

func (p *Scraper) scrape() string {
	if err := p.c.Visit(fmt.Sprintf("https://youtube.com/channel/%s/live?hl=en", p.cfg.Notifier.Platforms.YouTube.Channel)); err != nil {
		return ""
	}

	return <-p.idChan
}
