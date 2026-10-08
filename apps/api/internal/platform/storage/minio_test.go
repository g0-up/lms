package storage

import (
	"context"
	"errors"
	"net/http"
	"net/url"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// noNetwork đếm mọi request; presign phải thuần cục bộ nên số request phải bằng 0.
type noNetwork struct{ calls atomic.Int32 }

func (n *noNetwork) RoundTrip(*http.Request) (*http.Response, error) {
	n.calls.Add(1)
	return nil, errors.New("không được gọi mạng khi ký URL")
}

func offlineStorage(t *testing.T, rt http.RoundTripper) *MinIO {
	t.Helper()
	s, err := NewMinIO(Config{
		Endpoint: "127.0.0.1:1", PublicEndpoint: "http://127.0.0.1:1", Region: "us-east-1", Bucket: "lms-media",
		AccessKey: "test-access", SecretKey: "test-secret-key", Transport: rt,
	})
	require.NoError(t, err)
	return s
}

func TestPresignOffline(t *testing.T) {
	rt := &noNetwork{}
	s := offlineStorage(t, rt)
	ctx := context.Background()

	putURL, headers, err := s.PresignPut(ctx, "media/video/2026/10/a.mp4", "video/mp4", 1048576, 15*time.Minute)
	require.NoError(t, err)
	assert.Equal(t, map[string]string{"Content-Type": "video/mp4"}, headers)
	u, err := url.Parse(putURL)
	require.NoError(t, err)
	assert.Equal(t, "127.0.0.1:1", u.Host)
	assert.Equal(t, "/lms-media/media/video/2026/10/a.mp4", u.Path)
	assert.NotEmpty(t, u.Query().Get("X-Amz-Signature"))
	assert.Equal(t, "900", u.Query().Get("X-Amz-Expires"))
	signed := u.Query().Get("X-Amz-SignedHeaders")
	assert.Contains(t, signed, "content-type")
	assert.Contains(t, signed, "content-length")

	getURL, err := s.PresignGet(ctx, "media/video/2026/10/a.mp4", 2*time.Hour, "video/mp4")
	require.NoError(t, err)
	g, err := url.Parse(getURL)
	require.NoError(t, err)
	assert.NotEmpty(t, g.Query().Get("X-Amz-Signature"))
	assert.Equal(t, "7200", g.Query().Get("X-Amz-Expires"))
	assert.Equal(t, "video/mp4", g.Query().Get("response-content-type"))

	assert.Zero(t, rt.calls.Load(), "presign đã gọi mạng")
}

func TestPresignUsesPublicEndpoint(t *testing.T) {
	s, err := NewMinIO(Config{
		Endpoint: "minio:9000", PublicEndpoint: "https://media.example.com", Region: "auto", Bucket: "lms-media",
		AccessKey: "a", SecretKey: "b", Transport: &noNetwork{},
	})
	require.NoError(t, err)
	got, err := s.PresignGet(context.Background(), "media/image/2026/10/x.png", time.Minute, "")
	require.NoError(t, err)
	assert.True(t, strings.HasPrefix(got, "https://media.example.com/lms-media/media/image/2026/10/x.png?"), got)
	assert.NotContains(t, got, "response-content-type")
}

func TestNewMinIORejectsBadPublicEndpoint(t *testing.T) {
	for _, ep := range []string{"", "localhost:9000", "ftp://x", "http://x/sub"} {
		_, err := NewMinIO(Config{Endpoint: "minio:9000", PublicEndpoint: ep, Region: "us-east-1", Bucket: "b"})
		assert.Error(t, err, ep)
	}
}

func TestStatNetworkErrorIsNotNotFound(t *testing.T) {
	s := offlineStorage(t, &noNetwork{})
	_, err := s.Stat(context.Background(), "media/video/x.mp4")
	require.Error(t, err)
	assert.NotErrorIs(t, err, ErrNotFound)
}

func TestMemStorage(t *testing.T) {
	s := NewMemStorage()
	ctx := context.Background()
	_, err := s.Stat(ctx, "k")
	require.ErrorIs(t, err, ErrNotFound)

	require.NoError(t, s.Put(ctx, "k", strings.NewReader("hello"), 5, "image/png"))
	st, err := s.Stat(ctx, "k")
	require.NoError(t, err)
	assert.Equal(t, int64(5), st.Size)
	assert.Equal(t, "image/png", st.ContentType)
	assert.NotEmpty(t, st.ETag)

	require.NoError(t, s.Delete(ctx, "k"))
	_, err = s.Stat(ctx, "k")
	require.ErrorIs(t, err, ErrNotFound)
}
