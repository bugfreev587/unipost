package platform

import (
	"reflect"
	"testing"
)

func TestMediaFromURLsMarksExternalOriginAndPreservesOrder(t *testing.T) {
	urls := []string{
		"https://media.example/first.jpg",
		"https://media.example/second.mp4",
	}

	got := MediaFromURLs(urls)
	want := []MediaItem{
		{URL: urls[0], Kind: MediaKindImage, Origin: MediaOriginExternal},
		{URL: urls[1], Kind: MediaKindVideo, Origin: MediaOriginExternal},
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("MediaFromURLs() = %#v, want %#v", got, want)
	}
}

func TestMediaFromURLsWithOriginMarksManagedOrigin(t *testing.T) {
	urls := []string{
		"https://r2.example/first.png",
		"https://r2.example/second.mov",
	}

	got := MediaFromURLsWithOrigin(urls, MediaOriginManaged)
	want := []MediaItem{
		{URL: urls[0], Kind: MediaKindImage, Origin: MediaOriginManaged},
		{URL: urls[1], Kind: MediaKindVideo, Origin: MediaOriginManaged},
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("MediaFromURLsWithOrigin() = %#v, want %#v", got, want)
	}
}
