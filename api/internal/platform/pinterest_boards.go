package platform

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"sort"
	"strings"
	"sync"
	"time"
)

const (
	pinterestBoardCacheTTL = 60 * time.Second
	pinterestBoardPageSize = 250
	pinterestBoardMaxPages = 8
	pinterestBoardMaxCount = 2000
)

type pinterestBoardOwner struct {
	Username string `json:"username"`
}

type pinterestBoardResource struct {
	ID    string              `json:"id"`
	Name  string              `json:"name"`
	Owner pinterestBoardOwner `json:"owner"`
}

type pinterestBoardCacheKey struct {
	AccountID   string
	Environment string
}

type pinterestBoardCacheEntry struct {
	TokenFingerprint string
	Username         string
	OwnedIDs         map[string]struct{}
	Complete         bool
	ExpiresAt        time.Time
}

type pinterestBoardCache struct {
	mu      sync.Mutex
	entries map[pinterestBoardCacheKey]pinterestBoardCacheEntry
}

func (a *PinterestAdapter) preflightBoard(ctx context.Context, accessToken, boardID string) error {
	metadata, _ := DispatchMetadataFromContext(ctx)
	key := pinterestBoardCacheKey{
		AccountID:   strings.TrimSpace(metadata.SocialAccountID),
		Environment: PinterestEnvironment(),
	}
	tokenFingerprint := pinterestTokenFingerprint(accessToken)
	entry, cached := a.loadPinterestBoardCache(key, tokenFingerprint)
	if !cached || strings.TrimSpace(entry.Username) == "" {
		account, err := a.fetchUserAccount(ctx, accessToken, "destination_preflight")
		if err != nil {
			return err
		}
		entry = pinterestBoardCacheEntry{
			TokenFingerprint: tokenFingerprint,
			Username:         strings.TrimSpace(account.Username),
			OwnedIDs:         make(map[string]struct{}),
			ExpiresAt:        a.pinterestNow().Add(pinterestBoardCacheTTL),
		}
		if entry.Username == "" {
			return pinterestBoardProofTemporaryFailure("missing_operation_username")
		}
		a.storePinterestBoardCache(key, entry)
	}

	direct, err := a.fetchPinterestBoard(ctx, accessToken, boardID)
	if err != nil {
		return err
	}
	if direct.ID != boardID {
		return pinterestBoardProofTemporaryFailure("direct_id_mismatch")
	}
	if pinterestUsernameEqual(direct.Owner.Username, entry.Username) {
		if entry.OwnedIDs == nil {
			entry.OwnedIDs = make(map[string]struct{})
		}
		entry.OwnedIDs[boardID] = struct{}{}
		a.storePinterestBoardCache(key, entry)
		return nil
	}
	if _, ok := entry.OwnedIDs[boardID]; ok {
		return nil
	}
	if entry.Complete {
		return pinterestBoardTargetFailure("board_not_accessible")
	}

	ownedIDs, complete, found, err := a.fetchOwnedPinterestBoardIDs(ctx, accessToken, entry.Username, boardID)
	if err != nil {
		return err
	}
	entry.OwnedIDs = ownedIDs
	entry.Complete = complete
	entry.ExpiresAt = a.pinterestNow().Add(pinterestBoardCacheTTL)
	a.storePinterestBoardCache(key, entry)
	if found {
		return nil
	}
	if complete {
		return pinterestBoardTargetFailure("board_not_accessible")
	}
	return pinterestBoardProofTemporaryFailure("ownership_unresolved")
}

func (a *PinterestAdapter) fetchPinterestBoard(ctx context.Context, accessToken, boardID string) (pinterestBoardResource, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, pinterestAPIBaseURL()+"/boards/"+url.PathEscape(boardID), nil)
	if err != nil {
		return pinterestBoardResource{}, err
	}
	req.Header.Set("Authorization", "Bearer "+accessToken)
	resp, err := a.client.Do(req)
	if err != nil {
		return pinterestBoardResource{}, pinterestTransportFailure("board preflight", err, "destination_preflight")
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != http.StatusOK {
		return pinterestBoardResource{}, pinterestProviderFailure("board preflight", resp.StatusCode, body, "destination_preflight")
	}
	var board pinterestBoardResource
	if err := json.Unmarshal(body, &board); err != nil {
		return pinterestBoardResource{}, pinterestBoardProofTemporaryFailure("direct_decode_failed")
	}
	return board, nil
}

func (a *PinterestAdapter) fetchOwnedPinterestBoardIDs(ctx context.Context, accessToken, username, targetID string) (map[string]struct{}, bool, bool, error) {
	ownedIDs := make(map[string]struct{})
	seenIDs := make(map[string]struct{})
	seenBookmarks := make(map[string]struct{})
	seenPages := make(map[string]struct{})
	bookmark := ""

	for page := 1; page <= pinterestBoardMaxPages; page++ {
		query := url.Values{}
		query.Set("page_size", fmt.Sprint(pinterestBoardPageSize))
		if bookmark != "" {
			query.Set("bookmark", bookmark)
		}
		req, err := http.NewRequestWithContext(ctx, http.MethodGet, pinterestAPIBaseURL()+"/boards?"+query.Encode(), nil)
		if err != nil {
			return nil, false, false, err
		}
		req.Header.Set("Authorization", "Bearer "+accessToken)
		resp, err := a.client.Do(req)
		if err != nil {
			return nil, false, false, pinterestTransportFailure("board collection", err, "destination_preflight")
		}
		body, _ := io.ReadAll(resp.Body)
		resp.Body.Close()
		if resp.StatusCode != http.StatusOK {
			return nil, false, false, pinterestProviderFailure("board collection", resp.StatusCode, body, "destination_preflight")
		}
		var result struct {
			Items    []pinterestBoardResource `json:"items"`
			Bookmark string                   `json:"bookmark"`
		}
		if err := json.Unmarshal(body, &result); err != nil {
			return nil, false, false, pinterestBoardProofTemporaryFailure("collection_decode_failed")
		}
		if len(result.Items) == 0 && result.Bookmark != "" {
			return nil, false, false, pinterestBoardProofTemporaryFailure("empty_page_with_bookmark")
		}

		pageIDs := make([]string, 0, len(result.Items))
		for _, board := range result.Items {
			id := strings.TrimSpace(board.ID)
			if id == "" {
				continue
			}
			pageIDs = append(pageIDs, id)
			if _, seen := seenIDs[id]; !seen {
				seenIDs[id] = struct{}{}
				if len(seenIDs) > pinterestBoardMaxCount {
					return nil, false, false, pinterestBoardProofTemporaryFailure("board_count_cap")
				}
			}
			if pinterestUsernameEqual(board.Owner.Username, username) {
				ownedIDs[id] = struct{}{}
				if id == targetID {
					return ownedIDs, false, true, nil
				}
			}
		}
		sort.Strings(pageIDs)
		pageHash := sha256.Sum256([]byte(strings.Join(pageIDs, "\x00")))
		pageSignature := hex.EncodeToString(pageHash[:])
		if _, seen := seenPages[pageSignature]; seen {
			return nil, false, false, pinterestBoardProofTemporaryFailure("duplicate_page")
		}
		seenPages[pageSignature] = struct{}{}

		nextBookmark := strings.TrimSpace(result.Bookmark)
		if nextBookmark == "" {
			return ownedIDs, true, false, nil
		}
		if _, seen := seenBookmarks[nextBookmark]; seen {
			return nil, false, false, pinterestBoardProofTemporaryFailure("repeated_bookmark")
		}
		seenBookmarks[nextBookmark] = struct{}{}
		if page == pinterestBoardMaxPages {
			return nil, false, false, pinterestBoardProofTemporaryFailure("page_cap")
		}
		bookmark = nextBookmark
	}
	return nil, false, false, pinterestBoardProofTemporaryFailure("page_cap")
}

func pinterestBoardTargetFailure(reason string) error {
	return newProviderFailure(
		"The selected Pinterest board is unavailable for this connected account.",
		map[string]any{"provider": "pinterest", "reason": strings.TrimSpace(reason)},
		FailureContract{ErrorCode: "target_not_found", Stage: "destination_preflight", IsRetriable: false},
	)
}

func pinterestBoardProofTemporaryFailure(_ string) error {
	return newProviderFailure(
		"Pinterest could not verify the selected destination. Try again later.",
		map[string]any{"provider": "pinterest", "reason": "board_ownership_unverified", "is_transient": true},
		FailureContract{ErrorCode: "temporary_platform_error", Stage: "destination_preflight", IsRetriable: true},
	)
}

func pinterestUsernameEqual(left, right string) bool {
	left = strings.TrimSpace(left)
	right = strings.TrimSpace(right)
	return left != "" && right != "" && strings.EqualFold(left, right)
}

func pinterestTokenFingerprint(token string) string {
	sum := sha256.Sum256([]byte(token))
	return hex.EncodeToString(sum[:])
}

func (a *PinterestAdapter) pinterestNow() time.Time {
	if a.now != nil {
		return a.now()
	}
	return time.Now()
}

func (a *PinterestAdapter) loadPinterestBoardCache(key pinterestBoardCacheKey, tokenFingerprint string) (pinterestBoardCacheEntry, bool) {
	a.boardCache.mu.Lock()
	defer a.boardCache.mu.Unlock()
	entry, ok := a.boardCache.entries[key]
	if !ok || entry.TokenFingerprint != tokenFingerprint || !a.pinterestNow().Before(entry.ExpiresAt) {
		if ok {
			delete(a.boardCache.entries, key)
		}
		return pinterestBoardCacheEntry{}, false
	}
	entry.OwnedIDs = clonePinterestBoardIDs(entry.OwnedIDs)
	return entry, true
}

func (a *PinterestAdapter) storePinterestBoardCache(key pinterestBoardCacheKey, entry pinterestBoardCacheEntry) {
	a.boardCache.mu.Lock()
	defer a.boardCache.mu.Unlock()
	if a.boardCache.entries == nil {
		a.boardCache.entries = make(map[pinterestBoardCacheKey]pinterestBoardCacheEntry)
	}
	entry.OwnedIDs = clonePinterestBoardIDs(entry.OwnedIDs)
	a.boardCache.entries[key] = entry
}

func clonePinterestBoardIDs(source map[string]struct{}) map[string]struct{} {
	if len(source) == 0 {
		return make(map[string]struct{})
	}
	result := make(map[string]struct{}, len(source))
	for id := range source {
		result[id] = struct{}{}
	}
	return result
}

func (a *PinterestAdapter) InvalidateBoardCache(accountID string) {
	a.boardCache.mu.Lock()
	defer a.boardCache.mu.Unlock()
	for key := range a.boardCache.entries {
		if key.AccountID == strings.TrimSpace(accountID) {
			delete(a.boardCache.entries, key)
		}
	}
}
