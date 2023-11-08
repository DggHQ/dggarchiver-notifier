package yt

import (
	"errors"
	"fmt"

	config "github.com/DggHQ/dggarchiver-config/notifier"
	"google.golang.org/api/googleapi"
	"google.golang.org/api/youtube/v3"
)

var errIsNotModified = errors.New("not modified")

type errorWrapper struct {
	Message string
	Module  string
	Err     error
}

func (err *errorWrapper) Error() string {
	if err.Module == "" {
		return fmt.Sprintf("[YT] %s: %v", err.Message, err.Err)
	}

	return fmt.Sprintf("[YT] [%s] %s: %v", err.Module, err.Message, err.Err)
}

func (err *errorWrapper) Unwrap() error {
	return err.Err
}

func wrapWithYTError(err error, module string, message string) error {
	return &errorWrapper{
		Message: message,
		Module:  module,
		Err:     err,
	}
}

func getVideoInfo(cfg *config.Config, id string, etag string) ([]*youtube.Video, string, error) {
	resp, err := cfg.Notifier.Platforms.YouTube.Service.Videos.List([]string{"snippet", "liveStreamingDetails"}).IfNoneMatch(etag).Id(id).Do()
	if err != nil {
		if !googleapi.IsNotModified(err) {
			return nil, etag, wrapWithYTError(err, "", "Youtube API error")
		}
		return nil, etag, wrapWithYTError(errIsNotModified, "", "Got a 304 Not Modified for full video info, returning an empty slice")
	}

	return resp.Items, resp.Etag, nil
}
