package provider

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strconv"
	"strings"

	pluginv1 "github.com/Silo-Server/silo-plugin-sdk/pkg/pluginproto/silo/plugin/v1"
	"github.com/mildman1848/silo-plugin-sync-watcharr/internal/syncmap"
	"github.com/mildman1848/silo-plugin-sync-watcharr/internal/watcharr"
)

type WatcharrProvider struct {
	pluginv1.UnimplementedWatchSyncProviderServer
}

func (p *WatcharrProvider) InitAuthorize(context.Context, *pluginv1.WatchSyncInitAuthorizeRequest) (*pluginv1.WatchSyncInitAuthorizeResponse, error) {
	return &pluginv1.WatchSyncInitAuthorizeResponse{Fault: fault(pluginv1.WatchSyncFaultCode_WATCH_SYNC_FAULT_CODE_INVALID_REQUEST, "Watcharr uses manual login credentials for this plugin")}, nil
}

func (p *WatcharrProvider) ExchangeCode(context.Context, *pluginv1.WatchSyncExchangeCodeRequest) (*pluginv1.WatchSyncCredentialResponse, error) {
	return &pluginv1.WatchSyncCredentialResponse{Fault: fault(pluginv1.WatchSyncFaultCode_WATCH_SYNC_FAULT_CODE_INVALID_REQUEST, "Watcharr authorization-code flow is not supported")}, nil
}

func (p *WatcharrProvider) ExchangeAPIKey(ctx context.Context, req *pluginv1.WatchSyncExchangeAPIKeyRequest) (*pluginv1.WatchSyncCredentialResponse, error) {
	baseURL, err := baseURLFromConfig(req.GetProviderConfig())
	if err != nil {
		return &pluginv1.WatchSyncCredentialResponse{Fault: fault(pluginv1.WatchSyncFaultCode_WATCH_SYNC_FAULT_CODE_INVALID_REQUEST, err.Error())}, nil
	}
	token, err := tokenFromLoginSecret(ctx, baseURL, req.GetApiKey())
	if err != nil {
		return &pluginv1.WatchSyncCredentialResponse{Fault: fault(pluginv1.WatchSyncFaultCode_WATCH_SYNC_FAULT_CODE_INVALID_CREDENTIAL, err.Error())}, nil
	}
	client, err := watcharr.New(baseURL, token)
	if err != nil {
		return &pluginv1.WatchSyncCredentialResponse{Fault: fault(pluginv1.WatchSyncFaultCode_WATCH_SYNC_FAULT_CODE_INVALID_REQUEST, err.Error())}, nil
	}
	account, ferr := accountFromClient(ctx, client)
	if ferr != nil {
		return &pluginv1.WatchSyncCredentialResponse{Fault: ferr}, nil
	}
	return &pluginv1.WatchSyncCredentialResponse{
		Credentials: &pluginv1.WatchSyncCredentials{AccessToken: token, TokenType: "Bearer"},
		Account:     account,
	}, nil
}

func (p *WatcharrProvider) RefreshCredentials(_ context.Context, req *pluginv1.WatchSyncRefreshCredentialsRequest) (*pluginv1.WatchSyncCredentialResponse, error) {
	creds := req.GetContext().GetCredentials()
	if strings.TrimSpace(creds.GetAccessToken()) == "" {
		return &pluginv1.WatchSyncCredentialResponse{Fault: fault(pluginv1.WatchSyncFaultCode_WATCH_SYNC_FAULT_CODE_INVALID_CREDENTIAL, "Watcharr bearer token is missing")}, nil
	}
	return &pluginv1.WatchSyncCredentialResponse{Credentials: creds}, nil
}

func (p *WatcharrProvider) GetAccount(ctx context.Context, req *pluginv1.WatchSyncGetAccountRequest) (*pluginv1.WatchSyncGetAccountResponse, error) {
	client, err := clientFromAuthContext(req.GetContext())
	if err != nil {
		return &pluginv1.WatchSyncGetAccountResponse{Fault: fault(pluginv1.WatchSyncFaultCode_WATCH_SYNC_FAULT_CODE_INVALID_REQUEST, err.Error())}, nil
	}
	account, ferr := accountFromClient(ctx, client)
	if ferr != nil {
		return &pluginv1.WatchSyncGetAccountResponse{Fault: ferr}, nil
	}
	return &pluginv1.WatchSyncGetAccountResponse{Account: account}, nil
}

func (p *WatcharrProvider) ListRemoteState(ctx context.Context, req *pluginv1.WatchSyncListRemoteStateRequest) (*pluginv1.WatchSyncListRemoteStateResponse, error) {
	if req.GetPageToken() != "" {
		return &pluginv1.WatchSyncListRemoteStateResponse{CompleteSnapshot: true, NextCursor: "snapshot"}, nil
	}
	client, err := clientFromAuthContext(req.GetContext())
	if err != nil {
		return &pluginv1.WatchSyncListRemoteStateResponse{Fault: fault(pluginv1.WatchSyncFaultCode_WATCH_SYNC_FAULT_CODE_INVALID_REQUEST, err.Error())}, nil
	}
	items, err := client.ListWatched(ctx)
	if err != nil {
		return &pluginv1.WatchSyncListRemoteStateResponse{Fault: fault(pluginv1.WatchSyncFaultCode_WATCH_SYNC_FAULT_CODE_TEMPORARY, "failed to list Watcharr watched state")}, nil
	}
	return &pluginv1.WatchSyncListRemoteStateResponse{Items: syncmap.RemoteStateFromMedia(items), CompleteSnapshot: true, NextCursor: "snapshot"}, nil
}

func (p *WatcharrProvider) ApplyEvents(ctx context.Context, req *pluginv1.WatchSyncApplyEventsRequest) (*pluginv1.WatchSyncApplyEventsResponse, error) {
	client, err := clientFromAuthContext(req.GetContext())
	if err != nil {
		return &pluginv1.WatchSyncApplyEventsResponse{Fault: fault(pluginv1.WatchSyncFaultCode_WATCH_SYNC_FAULT_CODE_INVALID_REQUEST, err.Error())}, nil
	}
	results := make([]*pluginv1.WatchSyncApplyResult, 0, len(req.GetEvents()))
	for _, event := range req.GetEvents() {
		results = append(results, p.applyEvent(ctx, client, event))
	}
	return &pluginv1.WatchSyncApplyEventsResponse{Results: results}, nil
}

func (p *WatcharrProvider) applyEvent(ctx context.Context, client *watcharr.Client, event *pluginv1.WatchSyncEvent) *pluginv1.WatchSyncApplyResult {
	if event == nil {
		return applyResult("", pluginv1.WatchSyncApplyStatus_WATCH_SYNC_APPLY_STATUS_REJECTED, pluginv1.WatchSyncFaultCode_WATCH_SYNC_FAULT_CODE_INVALID_REQUEST, "empty watch sync event")
	}
	switch event.GetOperation() {
	case pluginv1.WatchSyncOperation_WATCH_SYNC_OPERATION_MARK_WATCHED:
		changed, err := markWatched(ctx, client, event.GetMedia())
		if err != nil {
			return applyResult(event.GetEventId(), pluginv1.WatchSyncApplyStatus_WATCH_SYNC_APPLY_STATUS_RETRY, pluginv1.WatchSyncFaultCode_WATCH_SYNC_FAULT_CODE_TEMPORARY, err.Error())
		}
		if changed {
			return applyResult(event.GetEventId(), pluginv1.WatchSyncApplyStatus_WATCH_SYNC_APPLY_STATUS_APPLIED, 0, "")
		}
		return applyResult(event.GetEventId(), pluginv1.WatchSyncApplyStatus_WATCH_SYNC_APPLY_STATUS_NO_CHANGE, 0, "")
	case pluginv1.WatchSyncOperation_WATCH_SYNC_OPERATION_MARK_UNWATCHED:
		return applyResult(event.GetEventId(), pluginv1.WatchSyncApplyStatus_WATCH_SYNC_APPLY_STATUS_REJECTED, pluginv1.WatchSyncFaultCode_WATCH_SYNC_FAULT_CODE_INVALID_REQUEST, "mark-unwatched export is intentionally disabled in v1")
	default:
		return applyResult(event.GetEventId(), pluginv1.WatchSyncApplyStatus_WATCH_SYNC_APPLY_STATUS_REJECTED, pluginv1.WatchSyncFaultCode_WATCH_SYNC_FAULT_CODE_INVALID_REQUEST, "unsupported watch sync operation")
	}
}

func markWatched(ctx context.Context, client *watcharr.Client, media *pluginv1.WatchSyncMedia) (bool, error) {
	if media == nil {
		return false, errors.New("event has no media")
	}
	items, err := client.ListWatched(ctx)
	if err != nil {
		return false, fmt.Errorf("list Watcharr state before apply: %w", err)
	}
	switch media.GetMediaType() {
	case pluginv1.WatchSyncMediaType_WATCH_SYNC_MEDIA_TYPE_MOVIE:
		tmdbID, ok := intExternalID(media.GetExternalIds(), "tmdb")
		if !ok {
			return false, errors.New("movie event has no tmdb external id")
		}
		if findMovie(items, tmdbID) != nil {
			return false, nil
		}
		_, err := client.AddMovie(ctx, tmdbID)
		return true, err
	case pluginv1.WatchSyncMediaType_WATCH_SYNC_MEDIA_TYPE_EPISODE:
		showTMDB, ok := intExternalID(media.GetSeriesExternalIds(), "tmdb")
		if !ok {
			return false, errors.New("episode event has no series tmdb external id")
		}
		season, episode := int(media.GetSeasonNumber()), int(media.GetEpisodeNumber())
		if season <= 0 || episode <= 0 {
			return false, errors.New("episode event has invalid season or episode number")
		}
		show := findShow(items, showTMDB)
		if show == nil {
			added, err := client.AddShow(ctx, showTMDB)
			if err != nil {
				return false, err
			}
			show = &watcharr.Media{IDs: watcharr.MediaIDs{TMDB: showTMDB}, Watched: &added}
		}
		if show.Watched == nil || show.Watched.ID == 0 {
			return false, errors.New("matched Watcharr show has no watched id")
		}
		if hasEpisode(show, season, episode) {
			return false, nil
		}
		return true, client.AddEpisode(ctx, show.Watched.ID, season, episode)
	default:
		return false, errors.New("unsupported media type")
	}
}

func findMovie(items []watcharr.Media, tmdbID int) *watcharr.Media {
	for i := range items {
		if (strings.EqualFold(items[i].Type, "tmdb_movie") || strings.EqualFold(items[i].Type, "movie")) && items[i].IDs.TMDB == tmdbID {
			return &items[i]
		}
	}
	return nil
}

func findShow(items []watcharr.Media, tmdbID int) *watcharr.Media {
	for i := range items {
		if (strings.EqualFold(items[i].Type, "tmdb_tv") || strings.EqualFold(items[i].Type, "tv")) && items[i].IDs.TMDB == tmdbID {
			return &items[i]
		}
	}
	return nil
}

func hasEpisode(show *watcharr.Media, season, episode int) bool {
	if show == nil || show.Watched == nil {
		return false
	}
	for _, ep := range show.Watched.WatchedEpisodes {
		if ep.SeasonNumber == season && ep.EpisodeNumber == episode {
			return true
		}
	}
	return false
}

func intExternalID(ids map[string]string, key string) (int, bool) {
	v := strings.TrimSpace(ids[key])
	if v == "" {
		return 0, false
	}
	n, err := strconv.Atoi(v)
	return n, err == nil && n > 0
}

func clientFromAuthContext(ctx *pluginv1.WatchSyncAuthenticatedContext) (*watcharr.Client, error) {
	if ctx == nil {
		return nil, errors.New("missing authenticated context")
	}
	return clientFromConfig(ctx.GetProviderConfig(), ctx.GetCredentials().GetAccessToken())
}

func clientFromConfig(cfg *pluginv1.WatchSyncProviderConfig, token string) (*watcharr.Client, error) {
	baseURL, err := baseURLFromConfig(cfg)
	if err != nil {
		return nil, err
	}
	return watcharr.New(baseURL, token)
}

func baseURLFromConfig(cfg *pluginv1.WatchSyncProviderConfig) (string, error) {
	if cfg == nil {
		return "", errors.New("missing provider config")
	}
	baseURL := firstConfigValue(cfg,
		"connection.base_url",
		"base_url",
	)
	if strings.TrimSpace(baseURL) == "" {
		return "", errors.New("watcharr base_url is required")
	}
	return baseURL, nil
}

type loginSecret struct {
	Username string `json:"username"`
	Password string `json:"password"`
	Token    string `json:"token"`
}

func tokenFromLoginSecret(ctx context.Context, baseURL, value string) (string, error) {
	value = strings.TrimSpace(value)
	if value == "" {
		return "", errors.New("Watcharr login secret is required")
	}
	if strings.HasPrefix(value, "eyJ") {
		return value, nil
	}
	if strings.HasPrefix(value, "{") {
		var secret loginSecret
		if err := json.Unmarshal([]byte(value), &secret); err != nil {
			return "", errors.New("Watcharr login secret JSON is invalid")
		}
		if strings.TrimSpace(secret.Token) != "" {
			return strings.TrimSpace(secret.Token), nil
		}
		return watcharr.Login(ctx, baseURL, secret.Username, secret.Password)
	}
	username, password, ok := strings.Cut(value, ":")
	if !ok {
		return "", errors.New("enter Watcharr credentials as username:password, JSON {\"username\":...,\"password\":...}, or an existing JWT")
	}
	return watcharr.Login(ctx, baseURL, username, password)
}

func firstConfigValue(cfg *pluginv1.WatchSyncProviderConfig, keys ...string) string {
	for _, key := range keys {
		if value := strings.TrimSpace(cfg.GetValues()[key]); value != "" {
			return value
		}
		if value := strings.TrimSpace(cfg.GetSecretValues()[key]); value != "" {
			return value
		}
	}
	return ""
}

func accountFromClient(ctx context.Context, client *watcharr.Client) (*pluginv1.WatchSyncAccount, *pluginv1.WatchSyncFault) {
	user, err := client.GetUser(ctx)
	if err != nil {
		return nil, fault(pluginv1.WatchSyncFaultCode_WATCH_SYNC_FAULT_CODE_INVALID_CREDENTIAL, "failed to authenticate with Watcharr")
	}
	username := strings.TrimSpace(user.Username)
	if username == "" {
		username = "watcharr"
	}
	return &pluginv1.WatchSyncAccount{ExternalSubject: username, Username: username, DisplayName: username}, nil
}

func applyResult(eventID string, status pluginv1.WatchSyncApplyStatus, code pluginv1.WatchSyncFaultCode, message string) *pluginv1.WatchSyncApplyResult {
	res := &pluginv1.WatchSyncApplyResult{EventId: eventID, Status: status}
	if code != 0 {
		res.Fault = fault(code, message)
	}
	return res
}

func fault(code pluginv1.WatchSyncFaultCode, message string) *pluginv1.WatchSyncFault {
	return &pluginv1.WatchSyncFault{Code: code, SafeMessage: message}
}
