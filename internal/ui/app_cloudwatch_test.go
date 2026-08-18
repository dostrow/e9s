package ui

import (
	"reflect"
	"testing"

	"github.com/dostrow/e9s/internal/config"
)

func TestSavedLogPathScopesSupportNewAndLegacyFields(t *testing.T) {
	path := config.LogPathEntry{
		LogGroup: "/aws/ecs/api", LogGroups: []string{"/aws/ecs/api", "/aws/ecs/worker"},
		Stream: "legacy", Streams: []string{"api/one", "api/two"},
	}
	if got := savedLogPathGroups(path); !reflect.DeepEqual(got, path.LogGroups) {
		t.Fatalf("groups = %#v", got)
	}
	if got := savedLogPathStreams(path); !reflect.DeepEqual(got, path.Streams) {
		t.Fatalf("streams = %#v", got)
	}

	legacy := config.LogPathEntry{LogGroup: "/aws/ecs/api", Stream: "api/one"}
	if got := savedLogPathGroups(legacy); !reflect.DeepEqual(got, []string{"/aws/ecs/api"}) {
		t.Fatalf("legacy groups = %#v", got)
	}
	if got := savedLogPathStreams(legacy); !reflect.DeepEqual(got, []string{"api/one"}) {
		t.Fatalf("legacy streams = %#v", got)
	}
}
