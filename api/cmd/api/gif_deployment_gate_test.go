package main

import (
	"io/fs"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/xiaoboyu/unipost-api/internal/db"
)

func TestDeploymentADoesNotRegisterGifConversionRoute(t *testing.T) {
	source, err := os.ReadFile("main.go")
	if err != nil {
		t.Fatalf("read main.go: %v", err)
	}
	if strings.Contains(strings.ToLower(string(source)), "/v1/media/gif-conversions") {
		t.Fatal("Deployment A must not register the GIF conversion route")
	}
}

func TestDeploymentAHasNoCallableGifJobInsertPath(t *testing.T) {
	paramsType := reflect.TypeOf(db.CreateMediaProcessingJobParams{})
	if _, exists := paramsType.FieldByName("InputMediaID"); exists {
		t.Fatal("generic media processing insert must not accept input_media_id during Deployment A")
	}

	err := filepath.WalkDir("../../internal", func(path string, entry fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if entry.IsDir() || filepath.Ext(path) != ".go" || strings.HasSuffix(path, "_test.go") {
			return nil
		}
		source, readErr := os.ReadFile(path)
		if readErr != nil {
			return readErr
		}
		if strings.Contains(strings.ToLower(string(source)), "gif_to_mp4") {
			t.Fatalf("Deployment A application code contains GIF job insertion literal in %s", path)
		}
		return nil
	})
	if err != nil {
		t.Fatalf("scan application source: %v", err)
	}
}
