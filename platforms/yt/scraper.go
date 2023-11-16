package yt

import (
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"regexp"
	"strings"
	"time"

	config "github.com/DggHQ/dggarchiver-config/notifier"
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
	prefix    slog.Attr
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
		c:      c,
		idChan: idChan,
		cfg:    cfg,
		state:  state,
		prefix: slog.Group("platform",
			slog.String("name", "youtube"),
			slog.String("method", "scrape"),
		),
		sleepTime: time.Second * 60 * time.Duration(cfg.Platforms.YouTube.ScraperRefresh),
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

// GetPrefix returns a slog.Attr group for platform p
func (p *Scraper) GetPrefix() slog.Attr {
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
				slog.Info("stream found",
					p.prefix,
					slog.String("id", id),
				)
				if p.cfg.Plugins.Enabled {
					util.LuaCallReceiveFunction(l, id)
				}
				vid, _, err := getVideoInfo(p.cfg, id, "")
				if err != nil && !errors.Is(err, errIsNotModified) {
					return err
				}

				vod := &dggarchivermodel.VOD{
					Platform:   "youtube",
					Downloader: p.cfg.Platforms.YouTube.Downloader,
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
					slog.Error("unable to marshal vod",
						p.prefix,
						slog.String("id", vod.ID),
						slog.Any("err", err),
					)
					return nil
				}

				if err = p.cfg.NATS.NatsConnection.Publish(fmt.Sprintf("%s.job", p.cfg.NATS.Topic), bytes); err != nil {
					slog.Error("unable to publish message",
						p.prefix,
						slog.String("id", vod.ID),
						slog.Any("err", err),
					)
					return nil
				}

				if p.cfg.Plugins.Enabled {
					util.LuaCallSendFunction(l, vod)
				}
				p.state.SentVODs = append(p.state.SentVODs, fmt.Sprintf("youtube:%s", vod.ID))
				p.state.Dump()
			} else {
				slog.Info("streaming on a different platform",
					p.prefix,
					slog.String("id", id),
				)
			}
		} else {
			slog.Info("already sent",
				p.prefix,
				slog.String("id", id),
			)
		}
	} else {
		p.state.CurrentStreams.YouTube = dggarchivermodel.VOD{}
		slog.Info("not live",
			p.prefix,
		)
	}

	util.HealthCheck(p.cfg.Platforms.YouTube.HealthCheck)

	return nil
}

func (p *Scraper) scrape() string {
	if err := p.c.Visit(fmt.Sprintf("https://youtube.com/channel/%s/live?hl=en", p.cfg.Platforms.YouTube.Channel)); err != nil {
		return ""
	}

	return <-p.idChan
}
