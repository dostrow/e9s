package service

import (
	"archive/zip"
	"bytes"
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/dostrow/e9s/internal/model"
)

type fakeLambdaAPI struct {
	functions []model.LambdaFunction
	detail    *model.LambdaFunction
	err       error
	updated   string
	archive   []byte
}

func (f *fakeLambdaAPI) ListLambdaFunctions(context.Context) ([]model.LambdaFunction, error) {
	return append([]model.LambdaFunction(nil), f.functions...), f.err
}

func (f *fakeLambdaAPI) GetLambdaFunction(context.Context, string) (*model.LambdaFunction, error) {
	return f.detail, f.err
}

func (f *fakeLambdaAPI) ResolveEnvVars(_ context.Context, environment []model.EnvVar) []model.EnvVar {
	resolved := append([]model.EnvVar(nil), environment...)
	for i := range resolved {
		resolved[i].ResolvedValue = "resolved:" + resolved[i].Value
	}
	return resolved
}

func (f *fakeLambdaAPI) LambdaPackageType(context.Context, string) (string, error) {
	return "Zip", f.err
}

func (f *fakeLambdaAPI) DownloadLambdaCode(context.Context, string) (string, error) {
	return "/tmp/function", f.err
}

func (f *fakeLambdaAPI) UpdateLambdaCode(_ context.Context, name string, archive []byte) error {
	f.updated, f.archive = name, append([]byte(nil), archive...)
	return f.err
}

func TestLambdaListFiltersAndSorts(t *testing.T) {
	api := &fakeLambdaAPI{functions: []model.LambdaFunction{
		{Name: "zeta", Description: "worker"},
		{Name: "API-handler", Description: "request path"},
		{Name: "beta", Description: "api processor"},
	}}
	got, err := NewLambda(api).List(context.Background(), " API ")
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 2 || got[0].Name != "API-handler" || got[1].Name != "beta" {
		t.Fatalf("List() = %#v, want substring matches in deterministic order", got)
	}
}

func TestLambdaDetailAndEnvironment(t *testing.T) {
	api := &fakeLambdaAPI{detail: &model.LambdaFunction{
		Name:    "worker",
		EnvVars: []model.EnvVar{{Name: "ZETA", Value: "z"}, {Name: "ALPHA", Value: "a"}},
	}}
	service := NewLambda(api)
	environment, err := service.Environment(context.Background(), "worker", true)
	if err != nil {
		t.Fatal(err)
	}
	if len(environment) != 2 || environment[0].Name != "ALPHA" || environment[0].ResolvedValue != "resolved:a" {
		t.Fatalf("Environment() = %#v", environment)
	}
	if _, err := service.Detail(context.Background(), " "); err == nil || !strings.Contains(err.Error(), "name is required") {
		t.Fatalf("Detail(empty) error = %v", err)
	}
}

func TestLambdaCodeOperationsWrapContext(t *testing.T) {
	cause := errors.New("denied")
	service := NewLambda(&fakeLambdaAPI{err: cause})
	if _, err := service.PackageType(context.Background(), "worker"); !errors.Is(err, cause) || !strings.Contains(err.Error(), "worker") {
		t.Fatalf("PackageType() error = %v", err)
	}
	if err := service.UpdateCode(context.Background(), "worker", nil); err == nil || !strings.Contains(err.Error(), "empty") {
		t.Fatalf("UpdateCode(empty) error = %v", err)
	}

	api := &fakeLambdaAPI{}
	service = NewLambda(api)
	if err := service.UpdateCode(context.Background(), " worker ", []byte("zip")); err != nil {
		t.Fatal(err)
	}
	if api.updated != "worker" || string(api.archive) != "zip" {
		t.Fatalf("UpdateCode() sent name=%q archive=%q", api.updated, api.archive)
	}
}

func TestZipDirectoryPreservesRelativePaths(t *testing.T) {
	directory := t.TempDir()
	nested := filepath.Join(directory, "lib")
	if err := os.Mkdir(nested, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(nested, "handler.js"), []byte("exports.handler = () => {}"), 0o644); err != nil {
		t.Fatal(err)
	}
	archive, err := ZipDirectory(directory)
	if err != nil {
		t.Fatal(err)
	}
	reader, err := zip.NewReader(bytes.NewReader(archive), int64(len(archive)))
	if err != nil {
		t.Fatal(err)
	}
	if len(reader.File) != 1 || reader.File[0].Name != filepath.Join("lib", "handler.js") {
		t.Fatalf("archive entries = %#v", reader.File)
	}
}
