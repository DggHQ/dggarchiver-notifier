package yt

import (
	"encoding/json"
	"errors"
	"fmt"
	"time"

	config "github.com/DggHQ/dggarchiver-config/notifier"
	log "github.com/DggHQ/dggarchiver-logger"
	dggarchivermodel "github.com/DggHQ/dggarchiver-model"
	"github.com/DggHQ/dggarchiver-notifier/util"
	lua "github.com/yuin/gopher-lua"
	"golang.org/x/exp/slices"
	"google.golang.org/api/googleapi"
	"google.golang.org/api/youtube/v3"
)

type API struct {
	cfg       *config.Config
	state     *util.State
	prefix    string
	sleepTime time.Duration
}

// New returns a new YouTube API platform struct
func NewAPI(cfg *config.Config, state *util.State) *API {
	p := API{
		cfg:       cfg,
		state:     state,
		prefix:    "[YT] [API]",
		sleepTime: time.Second * 60 * time.Duration(cfg.Notifier.Platforms.YouTube.APIRefresh),
	}

	return &p
}

// GetPrefix returns a log prefix for platform p
func (p *API) GetPrefix() string {
	return p.prefix
}

// GetSleepTime returns sleep duration for platform p
func (p *API) GetSleepTime() time.Duration {
	return p.sleepTime
}

// CheckLivestream checks for an existing livestream on platform p,
// and, if found, publishes the info to NATS
func (p *API) CheckLivestream(l *lua.LState) error {
	vid, etagEnd, err := p.getLivestreamID(p.state.SearchETag)
	if err != nil {
		if errors.Is(err, errIsNotModified) {
			log.Infof("[YT] [API] Livestream ID info had identical Etag as before, skipping")
			return nil
		}
		return err
	}

	p.state.SearchETag = etagEnd
	p.state.Dump()

	if len(vid) > 0 {
		if !slices.Contains(p.state.SentVODs, fmt.Sprintf("youtube:%s", vid[0].Id)) {
			log.Infof("[YT] [API] Found a currently running stream with ID %s", vid[0].Id)
			if p.cfg.Notifier.Plugins.Enabled {
				util.LuaCallReceiveFunction(l, vid[0].Id)
			}
			vod := &dggarchivermodel.VOD{
				Platform:  "youtube",
				ID:        vid[0].Id,
				PubTime:   vid[0].Snippet.PublishedAt,
				Title:     vid[0].Snippet.Title,
				StartTime: vid[0].LiveStreamingDetails.ActualStartTime,
				EndTime:   vid[0].LiveStreamingDetails.ActualEndTime,
				Thumbnail: vid[0].Snippet.Thumbnails.Medium.Url,
			}

			p.state.CurrentStreams.YouTube = *vod

			bytes, err := json.Marshal(vod)
			if err != nil {
				log.Fatalf("[YT] [API] Couldn't marshal VOD with ID %s into a JSON object: %v", vod.ID, err)
			}

			if err = p.cfg.NATS.NatsConnection.Publish(fmt.Sprintf("%s.job", p.cfg.NATS.Topic), bytes); err != nil {
				log.Errorf("[YT] [API] Wasn't able to send message with VOD with ID %s: %v", vod.ID, err)
				return nil
			}

			if p.cfg.Notifier.Plugins.Enabled {
				util.LuaCallSendFunction(l, vod)
			}
			p.state.SentVODs = append(p.state.SentVODs, fmt.Sprintf("youtube:%s", vod.ID))
			p.state.Dump()
		} else {
			log.Infof("[YT] [API] Stream with ID %s was already sent", vid[0].Id)
		}
	} else {
		p.state.CurrentStreams.YouTube = dggarchivermodel.VOD{}
		log.Infof("[YT] [API] No stream found")
	}

	return nil
}

func (p *API) getLivestreamID(etag string) ([]*youtube.Video, string, error) {
	resp, err := p.cfg.Notifier.Platforms.YouTube.Service.Search.List([]string{"snippet"}).IfNoneMatch(etag).EventType("live").ChannelId(p.cfg.Notifier.Platforms.YouTube.Channel).Type("video").Do()
	if err != nil {
		if !googleapi.IsNotModified(err) {
			return nil, etag, wrapWithYTError(err, "API", "Youtube API error")
		}
		return nil, etag, wrapWithYTError(errIsNotModified, "API", "Got a 304 Not Modified for livestream ID, returning an empty slice")
	}

	if len(resp.Items) > 0 {
		id, _, err := getVideoInfo(p.cfg, resp.Items[0].Id.VideoId, "")
		if err != nil && !errors.Is(err, errIsNotModified) {
			return id, resp.Etag, nil
		}
		return id, resp.Etag, nil
	}

	return nil, resp.Etag, nil
}
