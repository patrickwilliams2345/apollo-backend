package reddit

import (
	"context"
	"encoding/base64"
	"fmt"
	"net/http"
	"net/url"
	"strings"
)

// defaultUserAgent is used for any request without an explicit per-account
// User-Agent. Authenticated calls always set their own via WithUserAgent
// (Reddit requires the UA to match the registered OAuth app's owner).
const defaultUserAgent = "apollo-backend-selfhost/0.1"

type Request struct {
	body               url.Values
	query              url.Values
	method             string
	token              string
	url                string
	auth               string
	userAgent          string
	webSessionCookie   string
	webSessionModhash  string
	tags               []string
	emptyResponseBytes int
	retry              bool
	client             *http.Client
}

type RequestOption func(*Request)

func NewRequest(opts ...RequestOption) *Request {
	req := &Request{
		body:   url.Values{},
		query:  url.Values{},
		method: "GET",
		url:    "",

		token: "",
		auth:  "",

		tags: nil,

		emptyResponseBytes: 0,
		retry:              true,
		client:             nil,
	}

	for _, opt := range opts {
		opt(req)
	}

	// raw_json=1 strips HTML-entity escaping from content payloads — useful
	// for /r/..., /api/info, /message/inbox, etc. Reddit's WAF (Fastly) on
	// www.reddit.com/api/v1/access_token rejects requests carrying this
	// param with a "blocked by network security" 403 HTML page, so we only
	// add it for oauth.reddit.com calls where it actually matters.
	if strings.HasPrefix(req.url, "https://oauth.reddit.com/") {
		req.query.Set("raw_json", "1")
	}

	return req
}

func (r *Request) HTTPRequest(ctx context.Context) (*http.Request, error) {
	if r.webSessionCookie != "" {
		r.token = ""
		r.auth = ""
		r.url = webSessionURL(r.url)
	}
	req, err := http.NewRequestWithContext(ctx, r.method, r.url, strings.NewReader(r.body.Encode()))
	req.URL.RawQuery = r.query.Encode()

	req.Header.Add("Accept", "application/json")
	ua := r.userAgent
	if ua == "" {
		ua = defaultUserAgent
	}
	req.Header.Add("User-Agent", ua)

	// http.NewRequestWithContext does not auto-set Content-Type for form
	// bodies (only http.PostForm does). Reddit's edge layer routes POSTs
	// without it to the HTML web frontend, which serves the "blocked by
	// network security" page — so set it explicitly whenever there's a body.
	if r.method == http.MethodPost && len(r.body) > 0 {
		req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	}

	if r.token != "" {
		req.Header.Add("Authorization", fmt.Sprintf("Bearer %s", r.token))
	}

	if r.auth != "" {
		req.Header.Add("Authorization", fmt.Sprintf("Basic %s", r.auth))
	}
	if r.webSessionCookie != "" {
		req.Header.Set("Cookie", r.webSessionCookie)
		if r.method != http.MethodGet && r.method != http.MethodHead && r.webSessionModhash != "" {
			req.Header.Set("X-Modhash", r.webSessionModhash)
		}
	}

	return req, err
}

func webSessionURL(raw string) string {
	u, err := url.Parse(raw)
	if err != nil || u.Host != "oauth.reddit.com" {
		return raw
	}
	u.Host = "www.reddit.com"
	if u.Path == "/api/v1/me" {
		u.Path = "/api/me.json"
	} else if !strings.HasPrefix(u.Path, "/api/") && !strings.HasSuffix(u.Path, ".json") {
		u.Path = strings.TrimSuffix(u.Path, "/") + ".json"
	}
	return u.String()
}

func WithTags(tags []string) RequestOption {
	return func(req *Request) {
		req.tags = tags
	}
}

func WithMethod(method string) RequestOption {
	return func(req *Request) {
		req.method = method
	}
}

func WithURL(url string) RequestOption {
	return func(req *Request) {
		req.url = url
	}
}

func WithBasicAuth(user, password string) RequestOption {
	return func(req *Request) {
		encoded := base64.StdEncoding.EncodeToString([]byte(user + ":" + password))
		req.auth = encoded
	}
}

func WithUserAgent(ua string) RequestOption {
	return func(req *Request) {
		req.userAgent = ua
	}
}

func WithToken(token string) RequestOption {
	return func(req *Request) {
		req.token = token
	}
}

func WithWebSession(cookie, modhash string) RequestOption {
	return func(req *Request) {
		req.webSessionCookie = cookie
		req.webSessionModhash = modhash
	}
}

func WithBody(key, val string) RequestOption {
	return func(req *Request) {
		req.body.Set(key, val)
	}
}

func WithQuery(key, val string) RequestOption {
	if val == "" {
		return func(req *Request) {}
	}

	return func(req *Request) {
		req.query.Set(key, val)
	}
}

func WithEmptyResponseBytes(bytes int) RequestOption {
	return func(req *Request) {
		req.emptyResponseBytes = bytes
	}
}

func WithRetry(retry bool) RequestOption {
	return func(req *Request) {
		req.retry = retry
	}
}

func WithClient(client *http.Client) RequestOption {
	return func(req *Request) {
		req.client = client
	}
}
