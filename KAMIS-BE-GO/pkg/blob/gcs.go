package blob

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"golang.org/x/oauth2"
	"golang.org/x/oauth2/google"
)

// readWriteScope is the narrowest scope that covers all three operations below;
// it grants no bucket administration.
const readWriteScope = "https://www.googleapis.com/auth/devstorage.read_write"

// gcsTimeout bounds a single object transfer. Uploads are capped at 10MB by the
// handler, so a minute is generous even on a slow link.
const gcsTimeout = 60 * time.Second

// GCS stores objects in a Google Cloud Storage bucket over the JSON API.
//
// It deliberately does not use cloud.google.com/go/storage: that client pulls
// the gRPC/gax/protobuf stack — 139 additional modules — to do the three
// operations below, which is at odds with this repo's standing position on
// dependency weight (see MIGRATION.md on why gin is pinned). Authentication is
// still delegated to Google's own library, which is the part worth not
// hand-rolling.
type GCS struct {
	bucket  string
	baseURL string // overridden in tests
	http    *http.Client
}

// NewGCS builds a store over bucket, authenticating with Application Default
// Credentials. On a GCE VM those come from the instance metadata server, so
// there is no service-account key file to deploy or rotate; locally they come
// from GOOGLE_APPLICATION_CREDENTIALS or `gcloud auth application-default
// login`. The returned client refreshes access tokens on its own.
func NewGCS(ctx context.Context, bucket string) (*GCS, error) {
	if bucket == "" {
		return nil, fmt.Errorf("no GCS bucket configured")
	}
	source, err := google.DefaultTokenSource(ctx, readWriteScope)
	if err != nil {
		return nil, fmt.Errorf("google credentials: %w", err)
	}
	client := oauth2.NewClient(ctx, source)
	client.Timeout = gcsTimeout
	return &GCS{bucket: bucket, baseURL: "https://storage.googleapis.com", http: client}, nil
}

// objectURL builds the URL for one object. url.PathEscape escapes "/" as %2F,
// which is what the JSON API expects for an object name inside the path.
func (g *GCS) objectURL(key string, query string) string {
	u := fmt.Sprintf("%s/storage/v1/b/%s/o/%s",
		g.baseURL, url.PathEscape(g.bucket), url.PathEscape(key))
	if query != "" {
		u += "?" + query
	}
	return u
}

func (g *GCS) Put(ctx context.Context, key, contentType string, r io.Reader) error {
	if err := checkKey(key); err != nil {
		return err
	}
	if contentType == "" {
		contentType = "application/octet-stream"
	}

	// uploadType=media is the single-request upload: the body is the object
	// itself, with no multipart wrapper or metadata.
	endpoint := fmt.Sprintf("%s/upload/storage/v1/b/%s/o?uploadType=media&name=%s",
		g.baseURL, url.PathEscape(g.bucket), url.QueryEscape(key))

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, r)
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", contentType)

	res, err := g.http.Do(req)
	if err != nil {
		return fmt.Errorf("upload object: %w", err)
	}
	defer drain(res)

	if res.StatusCode >= 400 {
		return fmt.Errorf("upload object: GCS returned %d", res.StatusCode)
	}
	return nil
}

func (g *GCS) Get(ctx context.Context, key string) (*Object, error) {
	if err := checkKey(key); err != nil {
		return nil, err
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, g.objectURL(key, "alt=media"), nil)
	if err != nil {
		return nil, err
	}

	res, err := g.http.Do(req)
	if err != nil {
		return nil, fmt.Errorf("download object: %w", err)
	}
	if res.StatusCode == http.StatusNotFound {
		drain(res)
		return nil, ErrNotFound
	}
	if res.StatusCode >= 400 {
		drain(res)
		return nil, fmt.Errorf("download object: GCS returned %d", res.StatusCode)
	}

	// The body is handed to the caller unread, so the image streams through to
	// the HTTP response instead of being buffered in memory.
	size, _ := strconv.ParseInt(res.Header.Get("Content-Length"), 10, 64)
	contentType := res.Header.Get("Content-Type")
	if contentType == "" {
		contentType = "application/octet-stream"
	}
	return &Object{Body: res.Body, ContentType: contentType, Size: size}, nil
}

func (g *GCS) Delete(ctx context.Context, key string) error {
	if err := checkKey(key); err != nil {
		return err
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodDelete, g.objectURL(key, ""), nil)
	if err != nil {
		return err
	}

	res, err := g.http.Do(req)
	if err != nil {
		return fmt.Errorf("delete object: %w", err)
	}
	defer drain(res)

	// Already gone is a success, so a retried cleanup does not fail.
	if res.StatusCode == http.StatusNotFound {
		return nil
	}
	if res.StatusCode >= 400 {
		return fmt.Errorf("delete object: GCS returned %d", res.StatusCode)
	}
	return nil
}

// drain closes a response body the caller does not read, consuming what is left
// first so the connection can be reused.
func drain(res *http.Response) {
	_, _ = io.Copy(io.Discard, io.LimitReader(res.Body, 4096))
	_ = res.Body.Close()
}

// ContentTypeFor maps an upload's declared type onto what we will store,
// rejecting anything that is not an image. The Java service accepted any
// MultipartFile, so this is stricter: an HTML file served back from
// /asset/{id}/foto with its own content type would run as script on the
// frontend's origin.
func ContentTypeFor(declared string) (string, bool) {
	declared = strings.ToLower(strings.TrimSpace(declared))
	if i := strings.IndexByte(declared, ';'); i >= 0 {
		declared = strings.TrimSpace(declared[:i])
	}
	switch declared {
	case "image/jpeg", "image/jpg":
		return "image/jpeg", true
	case "image/png":
		return "image/png", true
	case "image/webp":
		return "image/webp", true
	case "image/gif":
		return "image/gif", true
	default:
		return "", false
	}
}

// ExtensionFor returns the filename extension for a type ContentTypeFor
// accepted, so generated keys carry one and the disk backend can recover the
// content type from the key alone.
func ExtensionFor(contentType string) string {
	switch contentType {
	case "image/jpeg":
		return ".jpg"
	case "image/png":
		return ".png"
	case "image/webp":
		return ".webp"
	case "image/gif":
		return ".gif"
	default:
		return ".bin"
	}
}
