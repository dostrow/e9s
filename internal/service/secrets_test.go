package service

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/dostrow/e9s/internal/model"
)

type fakeSecretsAPI struct {
	secrets     []model.Secret
	value       *model.SecretValue
	err         error
	createdName string
	created     string
	updatedName string
	updated     string
}

func (f *fakeSecretsAPI) ListSecrets(context.Context) ([]model.Secret, error) {
	return append([]model.Secret(nil), f.secrets...), f.err
}

func (f *fakeSecretsAPI) GetSecretValueByName(context.Context, string) (*model.SecretValue, error) {
	return f.value, f.err
}

func (f *fakeSecretsAPI) CreateSecret(_ context.Context, name, value, _ string) error {
	f.createdName, f.created = name, value
	return f.err
}

func (f *fakeSecretsAPI) PutSecretValue(_ context.Context, name, value string) error {
	f.updatedName, f.updated = name, value
	return f.err
}

func TestSecretsListFiltersSubstringAndSorts(t *testing.T) {
	api := &fakeSecretsAPI{secrets: []model.Secret{
		{Name: "zeta"},
		{Name: "Prod/API"},
		{Name: "api-token"},
	}}
	got, err := NewSecrets(api).List(context.Background(), " API ")
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 2 || got[0].Name != "api-token" || got[1].Name != "Prod/API" {
		t.Fatalf("List() = %#v, want case-insensitive substring matches in name order", got)
	}
}

func TestSecretsDetailValidatesAndWrapsErrors(t *testing.T) {
	service := NewSecrets(&fakeSecretsAPI{})
	if _, err := service.Detail(context.Background(), " "); err == nil || !strings.Contains(err.Error(), "name is required") {
		t.Fatalf("Detail(empty) error = %v", err)
	}

	cause := errors.New("access denied")
	service = NewSecrets(&fakeSecretsAPI{err: cause})
	if _, err := service.Detail(context.Background(), "prod/api"); !errors.Is(err, cause) || !strings.Contains(err.Error(), `"prod/api"`) {
		t.Fatalf("Detail() error = %v, want contextual wrapped cause", err)
	}
}

func TestSecretsCreateAndUpdate(t *testing.T) {
	api := &fakeSecretsAPI{}
	service := NewSecrets(api)
	if err := service.Create(context.Background(), " copy ", "created-value", ""); err != nil {
		t.Fatal(err)
	}
	if api.createdName != "copy" || api.created != "created-value" {
		t.Fatalf("Create() sent name=%q value=%q", api.createdName, api.created)
	}
	if err := service.Update(context.Background(), " original ", "updated-value"); err != nil {
		t.Fatal(err)
	}
	if api.updatedName != "original" || api.updated != "updated-value" {
		t.Fatalf("Update() sent name=%q value=%q", api.updatedName, api.updated)
	}
}
