package yt

import (
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	config "github.com/DggHQ/dggarchiver-config"
	log "github.com/DggHQ/dggarchiver-logger"
	dggarchivermodel "github.com/DggHQ/dggarchiver-model"
	"github.com/DggHQ/dggarchiver-notifier/util"
	"github.com/gocolly/colly/v2"
	lua "github.com/yuin/gopher-lua"
	"golang.org/x/exp/slices"
	"google.golang.org/api/googleapi"
	"google.golang.org/api/youtube/v3"
)

func ScrapeLivestreamID(cfg *config.Config) string {
	var index int
	var id string
	c := colly.NewCollector()
	// disable cookie handling to bypass youtube consent screen
	c.DisableCookies()

	c.OnResponse(func(r *colly.Response) {
		index = strings.Index(string(r.Body), "Started streaming ")
	})

	c.OnHTML("link[href][rel='canonical']", func(h *colly.HTMLElement) {
		if index != -1 {
			id = ytRegexp.FindStringSubmatch(h.Attr("href"))[1]
		}
	})

	c.Visit(fmt.Sprintf("https://youtube.com/channel/%s/live?hl=en", cfg.Notifier.Platforms.YouTube.Channel))

	return id
}

func GetLivestreamID(cfg *config.Config, etag string) ([]*youtube.Video, string, error) {
	resp, err := cfg.Notifier.Platforms.YouTube.Service.Search.List([]string{"snippet"}).IfNoneMatch(etag).EventType("live").ChannelId(cfg.Notifier.Platforms.YouTube.Channel).Type("video").Do()
	if err != nil {
		if !googleapi.IsNotModified(err) {
			return nil, etag, WrapWithYTError(err, "API", "Youtube API error")
		} else {
			return nil, etag, WrapWithYTError(ErrIsNotModified, "API", "Got a 304 Not Modified for livestream ID, returning an empty slice")
		}
	}

	if len(resp.Items) > 0 {
		id, _, err := GetVideoInfo(cfg, resp.Items[0].Id.VideoId, "")
		if err != nil && !errors.Is(err, ErrIsNotModified) {
			return id, resp.Etag, nil
		}
		return id, resp.Etag, nil
	} else {
		return nil, resp.Etag, nil
	}
}

func GetVideoInfo(cfg *config.Config, id string, etag string) ([]*youtube.Video, string, error) {
	resp, err := cfg.Notifier.Platforms.YouTube.Service.Videos.List([]string{"snippet", "liveStreamingDetails"}).IfNoneMatch(etag).Id(id).Do()
	if err != nil {
		if !googleapi.IsNotModified(err) {
			return nil, etag, WrapWithYTError(err, "", "Youtube API error")
		} else {
			return nil, etag, WrapWithYTError(ErrIsNotModified, "", "Got a 304 Not Modified for full video info, returning an empty slice")
		}
	}

	return resp.Items, resp.Etag, nil
}

func GetLivestreamInfo(cfg *config.Config, id string, etag string) ([]*youtube.Video, string, error) {
	resp, err := cfg.Notifier.Platforms.YouTube.Service.Videos.List([]string{"liveStreamingDetails"}).IfNoneMatch(etag).Id(id).Do()
	if err != nil {
		if !googleapi.IsNotModified(err) {
			return nil, etag, WrapWithYTError(err, "", "Youtube API error")
		} else {
			return nil, etag, WrapWithYTError(ErrIsNotModified, "", "Got a 304 Not Modified for livestream info, returning an empty slice")
		}
	}

	return resp.Items, resp.Etag, nil
}

func LoopApiLivestream(cfg *config.Config, state *util.State, L *lua.LState) error {
	vid, etagEnd, err := GetLivestreamID(cfg, state.SearchETag)
	if err != nil && !errors.Is(err, ErrIsNotModified) {
		return err
	}
	state.SearchETag = etagEnd
	state.Dump()
	if len(vid) > 0 && !slices.Contains(state.SentVODs, fmt.Sprintf("youtube:%s", vid[0].Id)) {
		log.Infof("[YT] [API] Found a currently running stream with ID %s", vid[0].Id)
		if cfg.Notifier.Plugins.Enabled {
			util.LuaCallReceiveFunction(L, vid[0].Id)
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

		state.CurrentStreams.YouTube = *vod

		bytes, err := json.Marshal(vod)
		if err != nil {
			log.Fatalf("[YT] [API] Couldn't marshal VOD with ID %s into a JSON object: %v", vod.ID, err)
		}

		if err = cfg.NATS.NatsConnection.Publish(fmt.Sprintf("%s.job", cfg.NATS.Topic), bytes); err != nil {
			log.Errorf("[YT] [API] Wasn't able to send message with VOD with ID %s: %v", vod.ID, err)
			return nil
		}

		if cfg.Notifier.Plugins.Enabled {
			util.LuaCallSendFunction(L, vod)
		}
		state.SentVODs = append(state.SentVODs, fmt.Sprintf("youtube:%s", vod.ID))
		state.Dump()
	} else {
		state.CurrentStreams.YouTube = dggarchivermodel.VOD{}
		log.Infof("[YT] [API] No stream found")
	}
	return nil
}

func LoopScrapedLivestream(cfg *config.Config, state *util.State, L *lua.LState) error {
	id := ScrapeLivestreamID(cfg)
	if id != "" {
		if !slices.Contains(state.SentVODs, fmt.Sprintf("youtube:%s", id)) {
			if state.CheckPriority("YouTube", cfg) {
				log.Infof("[YT] [SCRAPER] Found a currently running stream with ID %s", id)
				if cfg.Notifier.Plugins.Enabled {
					util.LuaCallReceiveFunction(L, id)
				}
				vid, _, err := GetVideoInfo(cfg, id, "")
				if err != nil && !errors.Is(err, ErrIsNotModified) {
					return err
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

				state.CurrentStreams.YouTube = *vod

				bytes, err := json.Marshal(vod)
				if err != nil {
					log.Fatalf("[YT] [SCRAPER] Couldn't marshal VOD with ID %s into a JSON object: %v", vod.ID, err)
				}

				if err = cfg.NATS.NatsConnection.Publish(fmt.Sprintf("%s.job", cfg.NATS.Topic), bytes); err != nil {
					log.Errorf("[YT] [SCRAPER] Wasn't able to send message with VOD with ID %s: %v", vod.ID, err)
					return nil
				}

				if cfg.Notifier.Plugins.Enabled {
					util.LuaCallSendFunction(L, vod)
				}
				state.SentVODs = append(state.SentVODs, fmt.Sprintf("youtube:%s", vod.ID))
				state.Dump()
			} else {
				log.Infof("[YT] [SCRAPER] Stream with ID %s is being streamed on a different platform, skipping", id)
			}
		} else {
			log.Infof("[YT] [SCRAPER] Stream with ID %s was already sent", id)
		}
	} else {
		state.CurrentStreams.YouTube = dggarchivermodel.VOD{}
		log.Infof("[YT] [SCRAPER] No stream found")
	}
	return nil
}
