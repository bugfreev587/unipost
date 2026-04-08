package handler

import (
	"sort"
	"testing"

	"github.com/xiaoboyu/unipost-api/internal/platform"
)

// TestGroupForDispatch_Standalone — N standalone posts produce N
// single-element groups so they all run in parallel.
func TestGroupForDispatch_Standalone(t *testing.T) {
	posts := []platform.PlatformPostInput{
		{AccountID: "a"},
		{AccountID: "b"},
		{AccountID: "c"},
	}
	groups := groupForDispatch(posts)
	if len(groups) != 3 {
		t.Fatalf("expected 3 groups, got %d", len(groups))
	}
	for _, g := range groups {
		if len(g) != 1 {
			t.Errorf("expected group of size 1, got %d", len(g))
		}
	}
}

// TestGroupForDispatch_SimpleThread — three threaded posts on the
// same account collapse to one group with positions in order.
func TestGroupForDispatch_SimpleThread(t *testing.T) {
	posts := []platform.PlatformPostInput{
		{AccountID: "a", ThreadPosition: 1},
		{AccountID: "a", ThreadPosition: 2},
		{AccountID: "a", ThreadPosition: 3},
	}
	groups := groupForDispatch(posts)
	if len(groups) != 1 {
		t.Fatalf("expected 1 group, got %d", len(groups))
	}
	g := groups[0]
	if len(g) != 3 {
		t.Fatalf("expected group of 3, got %d", len(g))
	}
	for i, idx := range g {
		if posts[idx].ThreadPosition != i+1 {
			t.Errorf("group position %d: expected ThreadPosition %d, got %d", i, i+1, posts[idx].ThreadPosition)
		}
	}
}

// TestGroupForDispatch_OutOfOrderThread — input arrives in random
// order; output is sorted by ThreadPosition so the dispatch loop
// chains them correctly.
func TestGroupForDispatch_OutOfOrderThread(t *testing.T) {
	posts := []platform.PlatformPostInput{
		{AccountID: "a", ThreadPosition: 3},
		{AccountID: "a", ThreadPosition: 1},
		{AccountID: "a", ThreadPosition: 2},
	}
	groups := groupForDispatch(posts)
	if len(groups) != 1 {
		t.Fatalf("expected 1 group, got %d", len(groups))
	}
	g := groups[0]
	for i, idx := range g {
		if posts[idx].ThreadPosition != i+1 {
			t.Errorf("after sort, position %d should have ThreadPosition %d, got %d", i, i+1, posts[idx].ThreadPosition)
		}
	}
}

// TestGroupForDispatch_TwoThreads — two threads on different
// accounts produce two separate groups; they can run in parallel.
func TestGroupForDispatch_TwoThreads(t *testing.T) {
	posts := []platform.PlatformPostInput{
		{AccountID: "a", ThreadPosition: 1},
		{AccountID: "a", ThreadPosition: 2},
		{AccountID: "b", ThreadPosition: 1},
		{AccountID: "b", ThreadPosition: 2},
	}
	groups := groupForDispatch(posts)
	if len(groups) != 2 {
		t.Fatalf("expected 2 groups, got %d", len(groups))
	}
	// Groups returned in map-iteration order; sort by their account
	// id to make the assertion deterministic.
	sort.Slice(groups, func(i, j int) bool {
		return posts[groups[i][0]].AccountID < posts[groups[j][0]].AccountID
	})
	if posts[groups[0][0]].AccountID != "a" || posts[groups[1][0]].AccountID != "b" {
		t.Errorf("groups not partitioned by account: %#v", groups)
	}
}

// TestGroupForDispatch_MixedSingleAndThread — a thread on one account
// and standalone posts on others all coexist.
func TestGroupForDispatch_MixedSingleAndThread(t *testing.T) {
	posts := []platform.PlatformPostInput{
		{AccountID: "a", ThreadPosition: 1}, // thread
		{AccountID: "a", ThreadPosition: 2}, // thread
		{AccountID: "b"},                    // standalone
		{AccountID: "c"},                    // standalone
	}
	groups := groupForDispatch(posts)
	// 1 thread + 2 standalones = 3 groups
	if len(groups) != 3 {
		t.Fatalf("expected 3 groups, got %d (groups: %#v)", len(groups), groups)
	}
	// The thread group should have 2 entries; the others should be 1.
	threadCount := 0
	singleCount := 0
	for _, g := range groups {
		if len(g) == 2 {
			threadCount++
		}
		if len(g) == 1 {
			singleCount++
		}
	}
	if threadCount != 1 {
		t.Errorf("expected exactly 1 thread group, got %d", threadCount)
	}
	if singleCount != 2 {
		t.Errorf("expected exactly 2 standalone groups, got %d", singleCount)
	}
}
