package syncmap

import (
	"strconv"
	"strings"

	pluginv1 "github.com/Bloem-Studios/bloem-plugin-sdk/pkg/pluginproto/silo/plugin/v1"
	"github.com/Bloem-Studios/bloem-community-mildman1848-sync-watcharr/internal/watcharr"
	"google.golang.org/protobuf/types/known/timestamppb"
)

func RemoteStateFromMedia(items []watcharr.Media) []*pluginv1.WatchSyncRemoteState {
	states := make([]*pluginv1.WatchSyncRemoteState, 0, len(items))
	for _, item := range items {
		if item.Watched == nil || !isWatchedStatus(item.Watched.Status) {
			continue
		}
		media := mediaFromWatcharr(item)
		if media == nil {
			continue
		}
		state := &pluginv1.WatchSyncRemoteState{
			ProviderItemKey: providerKey(item),
			Media:           media,
			Watched: &pluginv1.WatchSyncRemoteWatchedState{
				PlayCount: int32(max(1, item.Watched.Plays)),
			},
		}
		if !item.Watched.UpdatedAt.IsZero() {
			state.Watched.LastWatchedAt = timestamppb.New(item.Watched.UpdatedAt)
		}
		states = append(states, state)

		if strings.EqualFold(item.Type, "tmdb_tv") || strings.EqualFold(item.Type, "tv") {
			for _, ep := range item.Watched.WatchedEpisodes {
				if !isWatchedStatus(ep.Status) {
					continue
				}
				episodeMedia := &pluginv1.WatchSyncMedia{
					MediaType:         pluginv1.WatchSyncMediaType_WATCH_SYNC_MEDIA_TYPE_EPISODE,
					Title:             item.Name,
					ExternalIds:       map[string]string{},
					SeriesTitle:       item.Name,
					SeriesExternalIds: idMap(item.IDs),
					SeasonNumber:      int32(ep.SeasonNumber),
					EpisodeNumber:     int32(ep.EpisodeNumber),
					SeriesMediaItemId: strconv.Itoa(item.Watched.ID),
				}
				states = append(states, &pluginv1.WatchSyncRemoteState{
					ProviderItemKey: "watcharr:episode:" + strconv.Itoa(ep.ID),
					Media:           episodeMedia,
					Watched: &pluginv1.WatchSyncRemoteWatchedState{
						PlayCount: 1,
					},
				})
			}
		}
	}
	return states
}

func mediaFromWatcharr(item watcharr.Media) *pluginv1.WatchSyncMedia {
	switch strings.ToLower(item.Type) {
	case "tmdb_movie", "movie":
		return &pluginv1.WatchSyncMedia{MediaType: pluginv1.WatchSyncMediaType_WATCH_SYNC_MEDIA_TYPE_MOVIE, Title: item.Name, ExternalIds: idMap(item.IDs)}
	case "tmdb_tv", "tv":
		return &pluginv1.WatchSyncMedia{MediaType: pluginv1.WatchSyncMediaType_WATCH_SYNC_MEDIA_TYPE_EPISODE, SeriesTitle: item.Name, SeriesExternalIds: idMap(item.IDs)}
	default:
		return nil
	}
}

func idMap(ids watcharr.MediaIDs) map[string]string {
	out := map[string]string{}
	if ids.TMDB != 0 {
		out["tmdb"] = strconv.Itoa(ids.TMDB)
	}
	if ids.IMDB != "" {
		out["imdb"] = ids.IMDB
	}
	if ids.TVDB != 0 {
		out["tvdb"] = strconv.Itoa(ids.TVDB)
	}
	return out
}

func providerKey(item watcharr.Media) string {
	if item.Watched != nil && item.Watched.ID != 0 {
		return "watcharr:watched:" + strconv.Itoa(item.Watched.ID)
	}
	if item.IDs.TMDB != 0 {
		return "tmdb:" + strconv.Itoa(item.IDs.TMDB)
	}
	if item.IDs.IMDB != "" {
		return "imdb:" + item.IDs.IMDB
	}
	return "watcharr:title:" + strings.ToLower(item.Name)
}

func isWatchedStatus(status string) bool {
	return strings.EqualFold(status, "FINISHED") || strings.EqualFold(status, "COMPLETED") || strings.EqualFold(status, "WATCHED")
}

func max(a, b int) int {
	if a > b {
		return a
	}
	return b
}
