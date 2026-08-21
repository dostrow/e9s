//go:build gui

package gui

import (
	"reflect"
	"testing"

	"github.com/dostrow/e9s/internal/model"
)

func TestHangingIndentTagLeavesFirstLineAtViewportEdge(t *testing.T) {
	tag := newHangingIndentTag(137)
	if got := tag.ObjectProperty("left-margin"); got != 0 {
		t.Fatalf("left-margin = %#v, want 0", got)
	}
	if got := tag.ObjectProperty("indent"); got != -137 {
		t.Fatalf("indent = %#v, want -137", got)
	}
}

func TestTaskContainerNamesSkipsUnnamedContainers(t *testing.T) {
	task := model.Task{Containers: []model.Container{{Name: "api"}, {}, {Name: "sidecar"}}}
	if got, want := taskContainerNames(task), []string{"api", "sidecar"}; !reflect.DeepEqual(got, want) {
		t.Fatalf("taskContainerNames() = %#v, want %#v", got, want)
	}
}
