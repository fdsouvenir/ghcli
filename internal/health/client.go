package health

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	"golang.org/x/oauth2"
)

// Client is a small Google Health REST client.
type Client struct {
	base string
	http *http.Client
}

// NewClient constructs a Google Health client.
func NewClient(ctx context.Context, src oauth2.TokenSource) *Client {
	return &Client{
		base: APIBase,
		http: oauth2.NewClient(ctx, src),
	}
}

// Response captures a raw API response for archival.
type Response struct {
	Method     string
	Path       string
	Query      string
	StatusCode int
	Headers    http.Header
	Body       []byte
	FetchedAt  time.Time
}

// GetIdentity calls users/me/identity.
func (c *Client) GetIdentity(ctx context.Context) (Response, error) {
	return c.do(ctx, http.MethodGet, "/users/me/identity", nil, nil)
}

// GetProfile calls users/me/profile.
func (c *Client) GetProfile(ctx context.Context) (Response, error) {
	return c.do(ctx, http.MethodGet, "/users/me/profile", nil, nil)
}

// GetSettings calls users/me/settings.
func (c *Client) GetSettings(ctx context.Context) (Response, error) {
	return c.do(ctx, http.MethodGet, "/users/me/settings", nil, nil)
}

// ListDataPoints lists one data type page.
func (c *Client) ListDataPoints(ctx context.Context, dataType, filter, pageToken string, pageSize int) (Response, error) {
	q := url.Values{}
	if filter != "" {
		q.Set("filter", filter)
	}
	if pageToken != "" {
		q.Set("page_token", pageToken)
	}
	if pageSize > 0 {
		q.Set("page_size", fmt.Sprintf("%d", pageSize))
	}
	return c.do(ctx, http.MethodGet, "/users/me/dataTypes/"+url.PathEscape(dataType)+"/dataPoints", q, nil)
}

// DailyRollUp calls the dailyRollUp read endpoint for one data type.
func (c *Client) DailyRollUp(ctx context.Context, dataType string, start, end CivilTime, windowDays int) (Response, error) {
	body := map[string]any{
		"range": map[string]any{
			"start": start,
			"end":   end,
		},
		"windowSizeDays": windowDays,
	}
	return c.do(ctx, http.MethodPost, "/users/me/dataTypes/"+url.PathEscape(dataType)+"/dataPoints:dailyRollUp", nil, body)
}

// CivilTime is the JSON shape Google Health dailyRollUp expects.
type CivilTime struct {
	Date CivilDate  `json:"date"`
	Time CivilClock `json:"time"`
}

type CivilDate struct {
	Year  int `json:"year"`
	Month int `json:"month"`
	Day   int `json:"day"`
}

type CivilClock struct {
	Hours   int `json:"hours,omitempty"`
	Minutes int `json:"minutes,omitempty"`
	Seconds int `json:"seconds,omitempty"`
	Nanos   int `json:"nanos,omitempty"`
}

func CivilDayRange(day time.Time) (CivilTime, CivilTime) {
	y, m, d := day.Date()
	start := CivilTime{Date: CivilDate{Year: y, Month: int(m), Day: d}, Time: CivilClock{}}
	end := CivilTime{Date: CivilDate{Year: y, Month: int(m), Day: d}, Time: CivilClock{Hours: 23, Minutes: 59, Seconds: 59}}
	return start, end
}

func (c *Client) do(ctx context.Context, method, path string, query url.Values, body any) (Response, error) {
	var reader io.Reader
	if body != nil {
		b, err := json.Marshal(body)
		if err != nil {
			return Response{}, err
		}
		reader = bytes.NewReader(b)
	}
	u := c.base + path
	if len(query) > 0 {
		u += "?" + query.Encode()
	}
	req, err := http.NewRequestWithContext(ctx, method, u, reader)
	if err != nil {
		return Response{}, err
	}
	req.Header.Set("Accept", "application/json")
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}

	resp, err := c.http.Do(req)
	if err != nil {
		return Response{}, err
	}
	defer resp.Body.Close()
	b, err := io.ReadAll(resp.Body)
	if err != nil {
		return Response{}, err
	}
	r := Response{
		Method:     method,
		Path:       path,
		Query:      req.URL.RawQuery,
		StatusCode: resp.StatusCode,
		Headers:    resp.Header.Clone(),
		Body:       b,
		FetchedAt:  time.Now().UTC(),
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return r, fmt.Errorf("%s %s: HTTP %d: %s", method, path, resp.StatusCode, strings.TrimSpace(string(b)))
	}
	return r, nil
}
