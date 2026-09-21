package syncmap

import (
	"testing"

	pluginv1 "github.com/Bloem-Studios/bloem-plugin-sdk/pkg/pluginproto/silo/plugin/v1"
	"github.com/Bloem-Studios/bloem-community-mildman1848-sync-watcharr/internal/watcharr"
)

func TestRemoteStateFromMediaMapsMoviesAndEpisodes(t *testing.T) {
	items := []watcharr.Media{
		{Type: "tmdb_movie", Name: "Alien", IDs: watcharr.MediaIDs{TMDB: 348, IMDB: "tt0078748"}, Watched: &watcharr.Watched{ID: 10, Status: "FINISHED", Plays: 2}},
		{Type: "tmdb_tv", Name: "The Expanse", IDs: watcharr.MediaIDs{TMDB: 63639}, Watched: &watcharr.Watched{ID: 20, Status: "FINISHED", WatchedEpisodes: []watcharr.WatchedEpisode{{ID: 30, Status: "FINISHED", SeasonNumber: 1, EpisodeNumber: 1}}}},
	}
	states := RemoteStateFromMedia(items)
	if got, want := len(states), 3; got != want {
		t.Fatalf("len(states)=%d want %d", got, want)
	}
	if states[0].GetMedia().GetMediaType() != pluginv1.WatchSyncMediaType_WATCH_SYNC_MEDIA_TYPE_MOVIE {
		t.Fatalf("first state is not movie")
	}
	if states[0].GetWatched().GetPlayCount() != 2 {
		t.Fatalf("movie play count mismatch")
	}
	if states[2].GetMedia().GetMediaType() != pluginv1.WatchSyncMediaType_WATCH_SYNC_MEDIA_TYPE_EPISODE {
		t.Fatalf("third state is not episode")
	}
	if states[2].GetMedia().GetSeriesExternalIds()["tmdb"] != "63639" {
		t.Fatalf("series tmdb id not mapped")
	}
	if states[2].GetMedia().GetSeasonNumber() != 1 || states[2].GetMedia().GetEpisodeNumber() != 1 {
		t.Fatalf("episode numbers not mapped")
	}
}
