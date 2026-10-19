// Package cfsaas manages Cloudflare for SaaS custom hostnames of one zone through the Cloudflare API
// (https://developers.cloudflare.com/api/resources/custom_hostnames/).
package cfsaas

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
)

const DefaultBaseURL = "https://api.cloudflare.com/client/v4"

var ErrNotFound = errors.New("cloudflare: no such custom hostname")

type Client struct {
	// BaseURL is DefaultBaseURL unless a test points it elsewhere.
	BaseURL string
	ZoneID  string
	Token   string
	HTTP    *http.Client
}

type Hostname struct {
	ID       string
	Hostname string
	Status   string
	SSL      string
	Errors   []string
}

type apiError struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
}

type envelope struct {
	Success bool            `json:"success"`
	Errors  []apiError      `json:"errors"`
	Result  json.RawMessage `json:"result"`
}

type validationError struct {
	Message string `json:"message"`
}

type hostnameResult struct {
	ID       string `json:"id"`
	Hostname string `json:"hostname"`
	Status   string `json:"status"`
	SSL      *struct {
		Status           string            `json:"status"`
		ValidationErrors []validationError `json:"validation_errors"`
	} `json:"ssl"`
	VerificationErrors []string `json:"verification_errors"`
}

func (r hostnameResult) hostname() Hostname {
	h := Hostname{ID: r.ID, Hostname: r.Hostname, Status: r.Status, Errors: append([]string{}, r.VerificationErrors...)}
	if r.SSL != nil {
		h.SSL = r.SSL.Status
		for _, v := range r.SSL.ValidationErrors {
			if v.Message != "" {
				h.Errors = append(h.Errors, v.Message)
			}
		}
	}
	return h
}

// Error is an answer of the API that is not a success.
type Error struct {
	Status int
	Codes  []int
	Text   string
}

func (e *Error) Error() string {
	return fmt.Sprintf("cloudflare: %d %s", e.Status, e.Text)
}

func (c *Client) do(ctx context.Context, method, path string, query url.Values, body any, out any) error {
	base := c.BaseURL
	if base == "" {
		base = DefaultBaseURL
	}
	u := strings.TrimRight(base, "/") + "/zones/" + url.PathEscape(c.ZoneID) + "/custom_hostnames" + path
	if len(query) > 0 {
		u += "?" + query.Encode()
	}
	var rd io.Reader
	if body != nil {
		b, err := json.Marshal(body)
		if err != nil {
			return err
		}
		rd = bytes.NewReader(b)
	}
	req, err := http.NewRequestWithContext(ctx, method, u, rd)
	if err != nil {
		return err
	}
	req.Header.Set("Authorization", "Bearer "+c.Token)
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	hc := c.HTTP
	if hc == nil {
		hc = http.DefaultClient
	}
	res, err := hc.Do(req)
	if err != nil {
		return err
	}
	defer res.Body.Close()
	raw, err := io.ReadAll(io.LimitReader(res.Body, 4<<20))
	if err != nil {
		return err
	}
	var env envelope
	_ = json.Unmarshal(raw, &env)
	if res.StatusCode >= 300 || (len(raw) > 0 && !env.Success && len(env.Errors) > 0) {
		e := &Error{Status: res.StatusCode}
		var texts []string
		for _, ae := range env.Errors {
			e.Codes = append(e.Codes, ae.Code)
			texts = append(texts, fmt.Sprintf("%d %s", ae.Code, ae.Message))
		}
		if len(texts) == 0 {
			texts = append(texts, strings.TrimSpace(string(raw)))
		}
		e.Text = strings.Join(texts, "; ")
		if res.StatusCode == http.StatusNotFound {
			return errors.Join(ErrNotFound, e)
		}
		return e
	}
	if out != nil && len(env.Result) > 0 {
		return json.Unmarshal(env.Result, out)
	}
	return nil
}

// Create adds hostname with a DV certificate validated over HTTP. When Cloudflare refuses it and the
// zone has the hostname already (an earlier attempt whose answer was lost), that one is returned.
func (c *Client) Create(ctx context.Context, hostname string) (Hostname, error) {
	body := map[string]any{"hostname": hostname, "ssl": map[string]any{"method": "http", "type": "dv"}}
	var r hostnameResult
	err := c.do(ctx, http.MethodPost, "", nil, body, &r)
	var e *Error
	if errors.As(err, &e) && e.Status < 500 {
		h, ferr := c.Find(ctx, hostname)
		if ferr == nil {
			return h, nil
		}
		return Hostname{}, err
	}
	if err != nil {
		return Hostname{}, err
	}
	return r.hostname(), nil
}

func (c *Client) Get(ctx context.Context, id string) (Hostname, error) {
	var r hostnameResult
	if err := c.do(ctx, http.MethodGet, "/"+url.PathEscape(id), nil, nil, &r); err != nil {
		return Hostname{}, err
	}
	return r.hostname(), nil
}

// Find looks a hostname up by its name; ErrNotFound when the zone has none.
func (c *Client) Find(ctx context.Context, hostname string) (Hostname, error) {
	var rs []hostnameResult
	if err := c.do(ctx, http.MethodGet, "", url.Values{"hostname.exact": {hostname}}, nil, &rs); err != nil {
		return Hostname{}, err
	}
	for _, r := range rs {
		if strings.EqualFold(r.Hostname, hostname) {
			return r.hostname(), nil
		}
	}
	return Hostname{}, ErrNotFound
}

// Delete removes the custom hostname; one that is gone already is no error.
func (c *Client) Delete(ctx context.Context, id string) error {
	err := c.do(ctx, http.MethodDelete, "/"+url.PathEscape(id), nil, nil, nil)
	if errors.Is(err, ErrNotFound) {
		return nil
	}
	return err
}
