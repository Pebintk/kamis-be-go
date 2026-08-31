package httpx

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

// DefaultTimeout matches the 5s ceiling the Java ClientServiceImpl put on its
// cross-service project lookups.
const DefaultTimeout = 5 * time.Second

// Client is a small JSON client for service-to-service calls — the Go stand-in
// for Spring's WebClient. Every request forwards the caller's bearer token, as
// the Java services do via `headers.setBearerAuth(getTokenFromRequest())`.
//
// BaseURL is the *_URL env value (e.g. http://project-service:8083/api), so
// paths passed to the methods below are appended to it verbatim.
type Client struct {
	baseURL string
	http    *http.Client
}

func NewClient(baseURL string, timeout time.Duration) *Client {
	if timeout <= 0 {
		timeout = DefaultTimeout
	}
	return &Client{
		baseURL: strings.TrimSuffix(baseURL, "/"),
		http:    &http.Client{Timeout: timeout},
	}
}

// ErrNotFound reports a 404 from the downstream service. The Java code logs
// these and carries on with an empty result rather than failing the request, so
// callers are expected to treat it as "nothing there".
var ErrNotFound = fmt.Errorf("downstream resource not found")

func (c *Client) do(ctx context.Context, method, path string, body, out any) error {
	if c == nil || c.baseURL == "" {
		return fmt.Errorf("no base URL configured for this service")
	}

	var reader io.Reader
	if body != nil {
		encoded, err := json.Marshal(body)
		if err != nil {
			return fmt.Errorf("encode request body: %w", err)
		}
		reader = bytes.NewReader(encoded)
	}

	req, err := http.NewRequestWithContext(ctx, method, c.baseURL+path, reader)
	if err != nil {
		return err
	}
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	if token := TokenFromContext(ctx); token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}

	res, err := c.http.Do(req)
	if err != nil {
		return err
	}
	defer res.Body.Close()

	if res.StatusCode == http.StatusNotFound {
		return ErrNotFound
	}
	if res.StatusCode >= 400 {
		return fmt.Errorf("%s %s: downstream returned %d", method, path, res.StatusCode)
	}
	if out == nil {
		return nil
	}
	if err := json.NewDecoder(res.Body).Decode(out); err != nil {
		return fmt.Errorf("decode response: %w", err)
	}
	return nil
}

// Put sends a JSON body and ignores the response payload (the Java
// `bodyToMono(Void.class).block()` calls).
func (c *Client) Put(ctx context.Context, path string, body any) error {
	return c.do(ctx, http.MethodPut, path, body, nil)
}

// GetData issues a GET and returns the `data` field of the BaseResponseDTO
// envelope the KAMIS services wrap every response in.
//
// It is a package-level function rather than a method because Go does not allow
// type parameters on methods.
func GetData[T any](ctx context.Context, c *Client, path string) (T, error) {
	var envelope struct {
		Data T `json:"data"`
	}
	err := c.do(ctx, http.MethodGet, path, nil, &envelope)
	return envelope.Data, err
}

// ---- forwarded bearer token ----
//
// The token travels on the request context so handlers and services do not have
// to thread it through every signature — the same ergonomics the Java services
// got from injecting HttpServletRequest. pkg/auth's ForwardToken middleware is
// what puts it there.

type tokenKey struct{}

// ContextWithToken returns ctx carrying the raw bearer token to forward.
func ContextWithToken(ctx context.Context, raw string) context.Context {
	return context.WithValue(ctx, tokenKey{}, raw)
}

// TokenFromContext returns the raw bearer token on ctx, or "" if there is none.
func TokenFromContext(ctx context.Context) string {
	raw, _ := ctx.Value(tokenKey{}).(string)
	return raw
}
