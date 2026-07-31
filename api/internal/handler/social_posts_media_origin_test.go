package handler

import (
	"reflect"
	"testing"

	"github.com/xiaoboyu/unipost-api/internal/platform"
)

func TestMediaForDispatchPreservesExternalAndManagedOrigins(t *testing.T) {
	external := []string{
		"https://customer.example/first.jpg",
		"https://customer.example/second.mp4",
	}
	managed := []string{
		"https://r2.example/upload.png",
	}

	got := mediaForDispatch(external, managed)
	want := []platform.MediaItem{
		{URL: external[0], Kind: platform.MediaKindImage, Origin: platform.MediaOriginExternal},
		{URL: external[1], Kind: platform.MediaKindVideo, Origin: platform.MediaOriginExternal},
		{URL: managed[0], Kind: platform.MediaKindImage, Origin: platform.MediaOriginManaged},
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("mediaForDispatch() = %#v, want %#v", got, want)
	}
}
