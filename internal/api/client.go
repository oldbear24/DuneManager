package api

import (
"bufio"
"bytes"
"encoding/json"
"fmt"
"net/http"
"strings"

"github.com/oldbear24/DuneManager/internal/config"
)

// Client talks to the background service over HTTP.
type Client struct {
base string
http *http.Client
}

// NewClient builds a client using the current config port.
func NewClient() *Client {
return &Client{
base: "http://" + config.ServiceAddr(),
http: &http.Client{}, // zero Timeout = no timeout (streaming needs this)
}
}

// GetStatus fetches the current VM/service status.
func (c *Client) GetStatus() (*StatusResponse, error) {
resp, err := c.http.Get(c.base + "/api/status")
if err != nil {
return nil, err
}
defer resp.Body.Close()
var s StatusResponse
if err := json.NewDecoder(resp.Body).Decode(&s); err != nil {
return nil, err
}
return &s, nil
}

// Exec sends a command and streams output lines via onLine.
// Returns the "done" event Line field (e.g. a URL) on success.
func (c *Client) Exec(req ExecRequest, onLine func(string)) (string, error) {
body, _ := json.Marshal(req)
resp, err := c.http.Post(c.base+"/api/exec", "application/json", bytes.NewReader(body))
if err != nil {
return "", err
}
defer resp.Body.Close()

var lastResult, lastErr string
scanner := bufio.NewScanner(resp.Body)
for scanner.Scan() {
raw := scanner.Text()
if !strings.HasPrefix(raw, "data: ") {
continue
}
var evt SSEEvent
if json.Unmarshal([]byte(raw[6:]), &evt) != nil {
continue
}
switch evt.Type {
case "output":
onLine(evt.Line)
case "done":
lastResult = evt.Line
lastErr = evt.Error
}
}

if lastErr != "" {
return "", fmt.Errorf("%s", lastErr)
}
return lastResult, nil
}

// Kill asks the service to terminate the currently-running command.
func (c *Client) Kill() error {
resp, err := c.http.Post(c.base+"/api/kill", "application/json", nil)
if err != nil {
return err
}
resp.Body.Close()
return nil
}

// GetVersion returns the service binary version string.
func (c *Client) GetVersion() (string, error) {
	resp, err := c.http.Get(c.base + "/api/version")
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	var v VersionResponse
	if err := json.NewDecoder(resp.Body).Decode(&v); err != nil {
		return "", err
	}
	return v.Version, nil
}

// RestartService asks the service to restart itself.
func (c *Client) RestartService() error {
	resp, err := c.http.Post(c.base+"/api/service/restart", "application/json", nil)
	if err != nil {
		return err
	}
	resp.Body.Close()
	return nil
}

// CheckUpdate queries the service for update information.
func (c *Client) CheckUpdate() (*UpdateCheckResponse, error) {
	resp, err := c.http.Get(c.base + "/api/update/check")
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	var u UpdateCheckResponse
	if err := json.NewDecoder(resp.Body).Decode(&u); err != nil {
		return nil, err
	}
	if u.Error != "" {
		return nil, fmt.Errorf("%s", u.Error)
	}
	return &u, nil
}

// ApplyServiceUpdate streams the service update over SSE.
// Returns the GUI download URL (may be empty) when done.
func (c *Client) ApplyServiceUpdate(onLine func(string)) (string, error) {
	resp, err := c.http.Post(c.base+"/api/update/apply", "application/json", nil)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	var lastResult, lastErr string
	scanner := bufio.NewScanner(resp.Body)
	for scanner.Scan() {
		raw := scanner.Text()
		if !strings.HasPrefix(raw, "data: ") {
			continue
		}
		var evt SSEEvent
		if json.Unmarshal([]byte(raw[6:]), &evt) != nil {
			continue
		}
		switch evt.Type {
		case "output":
			onLine(evt.Line)
		case "done":
			lastResult = evt.Line
			lastErr = evt.Error
		}
	}
	if lastErr != "" {
		return "", fmt.Errorf("%s", lastErr)
	}
	return lastResult, nil
}

