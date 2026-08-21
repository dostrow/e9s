package service

import (
	"context"
	"errors"
	"reflect"
	"strings"
	"testing"

	"github.com/dostrow/e9s/internal/model"
)

type fakeSSMAPI struct {
	params      []model.Parameter
	detail      *model.Parameter
	err         error
	listedPath  string
	updatedName string
	updated     string
}

func (f *fakeSSMAPI) ListParameters(_ context.Context, path string) ([]model.Parameter, error) {
	f.listedPath = path
	return append([]model.Parameter(nil), f.params...), f.err
}

func (f *fakeSSMAPI) GetParameter(context.Context, string) (*model.Parameter, error) {
	return f.detail, f.err
}

func (f *fakeSSMAPI) PutParameter(_ context.Context, name, value string) error {
	f.updatedName, f.updated = name, value
	return f.err
}

func TestSSMListNormalizesAndSorts(t *testing.T) {
	api := &fakeSSMAPI{params: []model.Parameter{{Name: "/prod/z"}, {Name: "/prod/A"}}}
	got, err := NewSSM(api).List(context.Background(), " /prod/ ")
	if err != nil {
		t.Fatal(err)
	}
	if api.listedPath != "/prod" {
		t.Fatalf("listed path = %q", api.listedPath)
	}
	if names := []string{got[0].Name, got[1].Name}; !reflect.DeepEqual(names, []string{"/prod/A", "/prod/z"}) {
		t.Fatalf("sorted names = %#v", names)
	}
	if _, err := NewSSM(api).List(context.Background(), "prod"); err == nil {
		t.Fatal("path without a leading slash was accepted")
	}
}

func TestSSMDetailAndUpdateValidateAndRoute(t *testing.T) {
	api := &fakeSSMAPI{detail: &model.Parameter{Name: "/prod/api", Value: "secret"}}
	svc := NewSSM(api)
	got, err := svc.Detail(context.Background(), " /prod/api ")
	if err != nil || got.Value != "secret" {
		t.Fatalf("Detail() = %#v, %v", got, err)
	}
	if err := svc.Update(context.Background(), " /prod/api ", "new value"); err != nil {
		t.Fatal(err)
	}
	if api.updatedName != "/prod/api" || api.updated != "new value" {
		t.Fatalf("update route = %q, %q", api.updatedName, api.updated)
	}
	if _, err := svc.Detail(context.Background(), ""); err == nil {
		t.Fatal("empty detail name was accepted")
	}
}

func TestSSMWrapsAPIErrors(t *testing.T) {
	api := &fakeSSMAPI{err: errors.New("denied")}
	if _, err := NewSSM(api).List(context.Background(), "/"); err == nil || !strings.Contains(err.Error(), "list SSM parameters") {
		t.Fatalf("List() error = %v", err)
	}
	if err := NewSSM(api).Update(context.Background(), "/prod/api", "value"); err == nil || !strings.Contains(err.Error(), "update SSM parameter") {
		t.Fatalf("Update() error = %v", err)
	}
}
