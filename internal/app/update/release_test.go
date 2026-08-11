package update

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func stubGitHub(t *testing.T, handler http.HandlerFunc) {
	t.Helper()
	server := httptest.NewServer(handler)
	t.Cleanup(server.Close)
	previous := apiBase
	apiBase = server.URL + "/"
	t.Cleanup(func() { apiBase = previous })
}

func TestResolveTargetExactTagUsesServerTag(t *testing.T) {
	stubGitHub(t, func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "/repos/MSmaili/hetki/releases/tags/v1.2.3", r.URL.Path)
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{"tag_name": "v1.2.3", "draft": false}`))
	})

	tag, err := ResolveTarget(context.Background(), Options{TargetVersion: "v1.2.3"})
	require.NoError(t, err)
	assert.Equal(t, "v1.2.3", tag)
}

func TestResolveTargetExactTagRejectsMismatchedServerTag(t *testing.T) {
	stubGitHub(t, func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(`{"tag_name": "v9.9.9"}`))
	})

	_, err := ResolveTarget(context.Background(), Options{TargetVersion: "v1.2.3"})
	require.ErrorContains(t, err, `expected "v1.2.3"`)
}

func TestResolveTargetExactTagRejectsInvalidTagLocally(t *testing.T) {
	called := false
	stubGitHub(t, func(w http.ResponseWriter, r *http.Request) { called = true })

	_, err := ResolveTarget(context.Background(), Options{TargetVersion: "main"})
	require.ErrorContains(t, err, "must start with 'v'")
	assert.False(t, called, "invalid tags must fail before any network call")
}

func TestResolveTargetExactTagMissingReleaseFails(t *testing.T) {
	stubGitHub(t, func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
	})

	_, err := ResolveTarget(context.Background(), Options{TargetVersion: "v9.9.9"})
	require.ErrorContains(t, err, "v9.9.9 not found")
}

func TestResolveTargetExactTagDraftFails(t *testing.T) {
	stubGitHub(t, func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(`{"tag_name": "v1.2.3", "draft": true}`))
	})

	_, err := ResolveTarget(context.Background(), Options{TargetVersion: "v1.2.3"})
	require.ErrorContains(t, err, "draft")
}

func TestResolveTargetExactTagRequiresOptInForGitHubPrerelease(t *testing.T) {
	stubGitHub(t, func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(`{"tag_name":"v1.2.3","prerelease":true}`))
	})

	_, err := ResolveTarget(context.Background(), Options{TargetVersion: "v1.2.3"})
	require.ErrorContains(t, err, "--pre")
	tag, err := ResolveTarget(context.Background(), Options{TargetVersion: "v1.2.3", AllowPrerelease: true})
	require.NoError(t, err)
	assert.Equal(t, "v1.2.3", tag)
}

func TestResolveTargetLatestStable(t *testing.T) {
	stubGitHub(t, func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "/repos/MSmaili/hetki/releases", r.URL.Path)
		w.Write([]byte(`[{"tag_name": "v1.4.0", "prerelease": false},{"tag_name":"v1.3.0"}]`))
	})

	tag, err := ResolveTarget(context.Background(), Options{})
	require.NoError(t, err)
	assert.Equal(t, "v1.4.0", tag)
}

func TestResolveTargetLatestStableRejectsUnparsableTag(t *testing.T) {
	stubGitHub(t, func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(`[{"tag_name": "release-2026"}]`))
	})

	_, err := ResolveTarget(context.Background(), Options{})
	require.ErrorContains(t, err, "published release tag rejected")
}

func TestResolveTargetPrereleasePicksNewestOverall(t *testing.T) {
	stubGitHub(t, func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "/repos/MSmaili/hetki/releases", r.URL.Path)
		w.Write([]byte(`[
			{"tag_name": "v1.4.0-rc.2"},
			{"tag_name": "v1.4.0-rc.10"},
			{"tag_name": "v1.3.0"},
			{"tag_name": "v1.3.5", "draft": true}
		]`))
	})

	tag, err := ResolveTarget(context.Background(), Options{AllowPrerelease: true})
	require.NoError(t, err)
	assert.Equal(t, "v1.4.0-rc.10", tag, "numeric prerelease comparison, drafts skipped")
}

func TestResolveTargetPrereleaseAllTagsInvalidFails(t *testing.T) {
	stubGitHub(t, func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(`[{"tag_name": "nightly"}, {"tag_name": "v1"}]`))
	})

	_, err := ResolveTarget(context.Background(), Options{AllowPrerelease: true})
	require.ErrorContains(t, err, "published release tag rejected")
}

func TestResolveTargetListHTTPErrorFails(t *testing.T) {
	stubGitHub(t, func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	})

	_, err := ResolveTarget(context.Background(), Options{AllowPrerelease: true})
	require.ErrorContains(t, err, "listing releases")
}

func TestResolveTargetCancellationStopsResolution(t *testing.T) {
	stubGitHub(t, func(w http.ResponseWriter, r *http.Request) {
		<-r.Context().Done()
	})
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	_, err := ResolveTarget(ctx, Options{})
	require.ErrorIs(t, err, context.Canceled)
}

func TestReleaseMetadataSizeIsBounded(t *testing.T) {
	stubGitHub(t, func(w http.ResponseWriter, r *http.Request) {
		w.Write(make([]byte, maxMetadataBytes+1))
	})

	_, err := fetchLatestRelease(context.Background())
	require.ErrorContains(t, err, "exceeds")
}

func TestResolveTagCommitPeelsAnnotatedTag(t *testing.T) {
	commit := "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"
	stubGitHub(t, func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/repos/MSmaili/hetki/git/ref/tags/v1.2.3":
			w.Write([]byte(`{"object":{"sha":"bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb","type":"tag"}}`))
		case "/repos/MSmaili/hetki/git/tags/bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb":
			fmt.Fprintf(w, `{"object":{"sha":%q,"type":"commit"}}`, commit)
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	})

	got, err := resolveTagCommit(context.Background(), "v1.2.3")
	require.NoError(t, err)
	assert.Equal(t, commit, got)
}

func TestRedirectsToUnknownHostsAreRejected(t *testing.T) {
	evil := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Errorf("request reached disallowed host")
	}))
	defer evil.Close()
	stubGitHub(t, func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, evil.URL+"/binary", http.StatusFound)
	})

	_, err := fetchLatestRelease(context.Background())
	require.ErrorContains(t, err, "disallowed scheme")
}

func TestRedirectLoopTerminates(t *testing.T) {
	stubGitHub(t, func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, r.URL.String()+"x", http.StatusFound)
	})

	_, err := fetchLatestRelease(context.Background())
	require.Error(t, err)
}
