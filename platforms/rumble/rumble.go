package rumble

import (
	"encoding/json"
	"fmt"
	"sync/atomic"
	"time"

	config "github.com/DggHQ/dggarchiver-config/notifier"
	log "github.com/DggHQ/dggarchiver-logger"
	dggarchivermodel "github.com/DggHQ/dggarchiver-model"
	"github.com/DggHQ/dggarchiver-notifier/util"
	"github.com/gocolly/colly/v2"
	lua "github.com/yuin/gopher-lua"
	"golang.org/x/exp/slices"
)

type Platform struct {
	vodChan   chan *dggarchivermodel.VOD
	vodCheck  atomic.Bool
	c1        *colly.Collector
	c2        *colly.Collector
	cfg       *config.Config
	state     *util.State
	prefix    string
	sleepTime time.Duration
}

// New returns a new Rumble platform struct
func New(cfg *config.Config, state *util.State) *Platform {
	vodChan := make(chan *dggarchivermodel.VOD)

	c1 := colly.NewCollector()
	c1.DisableCookies()
	c1.AllowURLRevisit = true

	c2 := colly.NewCollector()
	c2.DisableCookies()
	c2.AllowURLRevisit = true

	p := Platform{
		vodChan:   vodChan,
		vodCheck:  atomic.Bool{},
		c1:        c1,
		c2:        c2,
		cfg:       cfg,
		state:     state,
		prefix:    "[Rumble] [SCRAPER]",
		sleepTime: time.Second * 60 * time.Duration(cfg.Notifier.Platforms.Rumble.ScraperRefresh),
	}

	c1.OnHTML("a.video-item--a", func(h *colly.HTMLElement) {
		go func() {
			if !p.wasVODSent() {
				live := h.ChildAttr("span.video-item--live", "data-value")
				p.setVODSent()
				if len(live) != 0 {
					link := h.Attr("href")
					embedData := getOEmbed(link)
					vodChan <- &dggarchivermodel.VOD{
						Platform:    "rumble",
						Downloader:  cfg.Notifier.Platforms.Rumble.Downloader,
						ID:          embedData.EmbedID,
						PlaybackURL: fmt.Sprintf("https://rumble.com%s", link),
						Title:       embedData.Title,
						StartTime:   time.Now().Format(time.RFC3339),
						EndTime:     "",
						Thumbnail:   embedData.Thumbnail,
					}
				} else {
					vodChan <- nil
				}
			}
		}()
	})

	c2.OnHTML("html", func(h *colly.HTMLElement) {
		go func() {
			if !p.wasVODSent() {
				liveDOM := h.DOM.Find(".watching-now")
				p.setVODSent()
				if len(liveDOM.Nodes) != 0 {
					linkDOM := h.DOM.Find("link[rel=canonical]")
					link, _ := linkDOM.Attr("href")
					embedData := getOEmbed(link)
					vodChan <- &dggarchivermodel.VOD{
						Platform:    "rumble",
						Downloader:  cfg.Notifier.Platforms.Rumble.Downloader,
						ID:          embedData.EmbedID,
						PlaybackURL: link,
						Title:       embedData.Title,
						StartTime:   time.Now().Format(time.RFC3339),
						EndTime:     "",
						Thumbnail:   embedData.Thumbnail,
					}
				} else {
					vodChan <- nil
				}
			}
		}()
	})

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
	vod := p.scrape()

	if vod != nil {
		if !slices.Contains(p.state.SentVODs, fmt.Sprintf("rumble:%s", vod.ID)) {
			if p.state.CheckPriority("Rumble", p.cfg) {
				log.Infof("[Rumble] [SCRAPER] Found a currently running stream with ID %s", vod.ID)
				if p.cfg.Notifier.Plugins.Enabled {
					util.LuaCallReceiveFunction(l, vod.ID)
				}

				p.state.CurrentStreams.Rumble = *vod

				bytes, err := json.Marshal(vod)
				if err != nil {
					log.Fatalf("[Rumble] [SCRAPER] Couldn't marshal VOD with ID %s into a JSON object: %v", vod.ID, err)
				}

				if err = p.cfg.NATS.NatsConnection.Publish(fmt.Sprintf("%s.job", p.cfg.NATS.Topic), bytes); err != nil {
					log.Errorf("[Rumble] [SCRAPER] Wasn't able to send message with VOD with ID %s: %v", vod.ID, err)
					return nil
				}

				if p.cfg.Notifier.Plugins.Enabled {
					util.LuaCallSendFunction(l, vod)
				}
				p.state.SentVODs = append(p.state.SentVODs, fmt.Sprintf("rumble:%s", vod.ID))
				p.state.Dump()
			} else {
				log.Infof("[Rumble] [SCRAPER] Stream with ID %s is being streamed on a different platform, skipping", vod.ID)
			}
		} else {
			log.Infof("[Rumble] [SCRAPER] Stream with ID %s was already sent", vod.ID)
		}
	} else {
		p.state.CurrentStreams.Rumble = dggarchivermodel.VOD{}
		log.Infof("[Rumble] [SCRAPER] No stream found")
	}
	return nil
}

func (p *Platform) wasVODSent() bool {
	return p.vodCheck.Load()
}

func (p *Platform) setVODSent() {
	p.vodCheck.Store(true)
}

func (p *Platform) resetVODSent() {
	p.vodCheck.Store(false)
}

func (p *Platform) scrape() *dggarchivermodel.VOD {
	p.resetVODSent()

	if err := p.c1.Visit(fmt.Sprintf("https://rumble.com/c/%s?date=today", p.cfg.Notifier.Platforms.Rumble.Channel)); err != nil {
		return nil
	}
	if err := p.c1.Visit(fmt.Sprintf("https://rumble.com/c/%s?date=this-week", p.cfg.Notifier.Platforms.Rumble.Channel)); err != nil {
		return nil
	}
	if err := p.c1.Visit(fmt.Sprintf("https://rumble.com/c/%s?date=this-month", p.cfg.Notifier.Platforms.Rumble.Channel)); err != nil {
		return nil
	}
	if err := p.c1.Visit(fmt.Sprintf("https://rumble.com/c/%s?date=this-year", p.cfg.Notifier.Platforms.Rumble.Channel)); err != nil {
		return nil
	}
	if err := p.c1.Visit(fmt.Sprintf("https://rumble.com/c/%s", p.cfg.Notifier.Platforms.Rumble.Channel)); err != nil {
		return nil
	}

	if err := p.c2.Visit(fmt.Sprintf("https://rumble.com/%s/live", p.cfg.Notifier.Platforms.Rumble.Channel)); err != nil {
		return nil
	}

	return <-p.vodChan
}
