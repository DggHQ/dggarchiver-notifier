package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"net/http"
	"time"
)

const (
	url = "https://www.destiny.gg/api"
)

/*
The DGG API Client
*/
type DGGApiClient struct {
	URL        string
	HTTPClient *http.Client
}

/*
Invoke new API Client
*/
func NewClient() *DGGApiClient {
	return &DGGApiClient{
		URL: url,
		HTTPClient: &http.Client{
			Timeout: time.Minute,
		},
	}
}

/*
Contains error Response data for failed API requests
*/
type errorResponse struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
}

/*
Helper function that parses the json to the provided interface to avoid code duplication
*/
func (c *DGGApiClient) sendRequest(req *http.Request, v interface{}) error {
	req.Header.Add("Content-Type", "application/json;charset=utf-8")
	// Do the request for the requested API endpoint
	res, err := c.HTTPClient.Do(req)
	if err != nil {
		return err
	}
	defer res.Body.Close()
	// Handle HTTP return codes
	if res.StatusCode < http.StatusOK || res.StatusCode >= http.StatusBadRequest {
		var errRes errorResponse
		if err = json.NewDecoder(res.Body).Decode(&errRes); err == nil {
			return errors.New(errRes.Message)
		}
		return fmt.Errorf("unknown error, status code: %d", res.StatusCode)
	}
	// Decode json into the interface passed from the endpoint's method
	if err = json.NewDecoder(res.Body).Decode(&v); err != nil {
		log.Println("Something went wrong")
		return err
	}
	return nil
}
