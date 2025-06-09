package tiktok

import (
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"os"
	"slices"
	"time"

	config "github.com/DggHQ/dggarchiver-config/notifier"
	dggarchivermodel "github.com/DggHQ/dggarchiver-model"
	"github.com/DggHQ/dggarchiver-notifier/notifications"
	"github.com/DggHQ/dggarchiver-notifier/platforms/implementation"
	"github.com/DggHQ/dggarchiver-notifier/state"
	"github.com/DggHQ/dggarchiver-notifier/util"
	"github.com/containrrr/shoutrrr/pkg/types"
	"github.com/steampoweredtaco/gotiktoklive"
)

const (
	platformName   string = "tiktok"
	platformMethod string = "scraper"
)

func init() {
	implementation.Map[fmt.Sprintf("%s_%s", platformName, platformMethod)] = New
}

type Platform struct {
	tt *gotiktoklive.TikTok

	cfg       *config.Config
	state     *state.State
	prefix    slog.Attr
	sleepTime time.Duration
}

// New returns a new TikTok platform struct
func New(cfg *config.Config, state *state.State) implementation.Platform {
	p := Platform{
		cfg:   cfg,
		state: state,
		prefix: slog.Group("platform",
			slog.String("name", platformName),
			slog.String("method", platformMethod),
		),
		sleepTime: time.Second * 60 * time.Duration(cfg.Platforms.TikTok.RefreshTime),
	}

	var opts []gotiktoklive.TikTokLiveOption
	if cfg.Platforms.TikTok.ProxyURL != "" {
		opts = append(opts, gotiktoklive.Proxy(cfg.Platforms.TikTok.ProxyURL, false))
	}

	tt, err := gotiktoklive.NewTikTok(opts...)
	if err != nil {
		slog.Error("unable to start tiktok", slog.Any("err", err))
		os.Exit(1)
	}

	p.tt = tt

	return &p
}

// GetPrefix returns a slog.Attr group for platform p
func (p *Platform) GetPrefix() slog.Attr {
	return p.prefix
}

// GetSleepTime returns sleep duration for platform p
func (p *Platform) GetSleepTime() time.Duration {
	return p.sleepTime
}

// CheckLivestream checks for an existing livestream on platform p,
// and, if found, publishes the info to NATS
func (p *Platform) CheckLivestream() error {
	stream := p.scrape()

	if stream != nil && stream.StreamID != 0 {
		slog.Debug("got stream", "stream", stream)
		if !slices.Contains(p.state.SentVODs, fmt.Sprintf("tiktok:%d", stream.StreamID)) {
			if p.state.CheckPriority("TikTok", p.cfg) {
				slog.Info("stream found",
					p.prefix,
					slog.Int("id", int(stream.StreamID)),
				)
				if p.cfg.Notifications.Condition("receive") {
					errs := p.cfg.Notifications.Sender.Send(notifications.GetReceiveMessage("TikTok", fmt.Sprint(stream.StreamID)), &types.Params{
						"title": "Received stream",
					})
					for _, err := range errs {
						if err != nil {
							slog.Warn("unable to send notification", p.prefix, slog.Int("id", int(stream.StreamID)), slog.Any("err", err))
						}
					}
				}

				vod := &dggarchivermodel.VOD{
					Platform:    "tiktok",
					Downloader:  p.cfg.Platforms.TikTok.Downloader,
					VID:         fmt.Sprintf("%d", stream.StreamID),
					PlaybackURL: fmt.Sprintf("https://tiktok.com/@%s/live", p.cfg.Platforms.TikTok.Channel),
					Title:       stream.Title,
					StartTime:   time.Now().Format(time.RFC3339),
					EndTime:     "",
					Thumbnail:   "",
					Quality:     p.cfg.Platforms.TikTok.Quality,
					Tags:        p.cfg.Platforms.TikTok.Tags,
					WorkerProxy: p.cfg.Platforms.TikTok.WorkerProxyURL,
				}

				p.state.CurrentStreams.TikTok = *vod

				bytes, err := json.Marshal(vod)
				if err != nil {
					slog.Error("unable to marshal vod",
						p.prefix,
						slog.String("id", vod.VID),
						slog.Any("err", err),
					)
					return nil
				}

				if err = p.cfg.NATS.NatsConnection.Publish(fmt.Sprintf("%s.job", p.cfg.NATS.Topic), bytes); err != nil {
					slog.Error("unable to publish message",
						p.prefix,
						slog.String("id", vod.VID),
						slog.Any("err", err),
					)
					return nil
				}

				if p.cfg.Notifications.Condition("send") {
					errs := p.cfg.Notifications.Sender.Send(notifications.GetSendMessage(vod), &types.Params{
						"title": "Sent stream",
					})
					for _, err := range errs {
						if err != nil {
							slog.Warn("unable to send notification", p.prefix, slog.String("id", vod.VID), slog.Any("err", err))
						}
					}
				}
				p.state.SentVODs = append(p.state.SentVODs, fmt.Sprintf("tiktok:%s", vod.VID))
				p.state.Dump()
			} else {
				slog.Info("streaming on a different platform",
					p.prefix,
					slog.Int("id", int(stream.StreamID)),
				)
			}
		} else {
			slog.Info("already sent",
				p.prefix,
				slog.Int("id", int(stream.StreamID)),
			)
		}
	} else {
		p.state.CurrentStreams.Kick = dggarchivermodel.VOD{}
		slog.Info("not live",
			p.prefix,
		)
	}

	util.HealthCheck(p.cfg.Platforms.Kick.HealthCheck)

	return nil
}

func (p *Platform) scrape() *gotiktoklive.RoomInfo {
	stream, err := p.tt.GetRoomInfo(p.cfg.Platforms.TikTok.Channel)
	if err != nil {
		switch {
		case !errors.Is(err, gotiktoklive.ErrUserOffline), !errors.Is(err, gotiktoklive.ErrLiveHasEnded):
		default:
			slog.Error("unable to get tiktok room info",
				p.prefix,
				slog.Any("err", err),
			)
		}
		return nil
	}

	return stream
}
