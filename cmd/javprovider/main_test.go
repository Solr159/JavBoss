package main

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"reflect"
	"strings"
	"testing"

	"javboss/internal/jav"
)

type unexpectedReader struct{}

func (unexpectedReader) Read([]byte) (int, error) { panic("CLI mode must not read stdin") }

func TestCommandLookupModes(t *testing.T) {
	for _, tc := range []struct {
		name          string
		args          []string
		provider      jav.Provider
		method, input string
	}{
		{"CLI default method", []string{"-provider", "javdb-api", "-input", "ABC-001"}, jav.ProviderJavDBAPI, "LookupJavByCode", "ABC-001"},
		{"CLI explicit method", []string{"--provider", "minnanoav", "--method", "LookupActressByJapaneseName", "--input", " 女优 名字 "}, jav.ProviderMinnanoAV, "LookupActressByJapaneseName", "女优 名字"},
		{"CLI case insensitive", []string{"-provider", "JAVDATABASE", "-method", "lookupactressbycode", "-input", "ABC-001"}, jav.ProviderJavDatabase, "LookupActressByCode", "ABC-001"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			cmd := newCommand()
			type contextKey struct{}
			ctx, cancel := context.WithCancel(context.WithValue(context.Background(), contextKey{}, "request"))
			defer cancel()
			calls := 0
			for i := range cmd.methods {
				method := cmd.methods[i].name
				cmd.methods[i].call = func(gotCtx context.Context, provider jav.Provider, input string) (any, error) {
					calls++
					if gotCtx.Value(contextKey{}) != "request" || provider != tc.provider || method != tc.method || input != tc.input {
						t.Errorf("unexpected lookup: context=%v provider=%s method=%s input=%q", gotCtx.Value(contextKey{}) == "request", provider, method, input)
					}
					return map[string]string{"result": "ok"}, nil
				}
			}
			var out, errOut bytes.Buffer
			if err := cmd.run(ctx, tc.args, unexpectedReader{}, &out, &errOut); err != nil {
				t.Fatal(err)
			}
			if calls != 1 || errOut.Len() != 0 {
				t.Fatalf("calls=%d stderr=%q", calls, errOut.String())
			}
			var result map[string]string
			if err := json.Unmarshal(out.Bytes(), &result); err != nil || result["result"] != "ok" {
				t.Fatalf("stdout must contain only the JSON result: %q, %v", out.String(), err)
			}
		})
	}
}

func TestCommandValidationAndHelp(t *testing.T) {
	for _, tc := range []struct {
		name      string
		args      []string
		wantError bool
	}{
		{"help", []string{"--help"}, false},
		{"legacy help", []string{"-help"}, false},
		{"short help", []string{"-h"}, false},
		{"missing provider", []string{"-input", "ABC-001"}, true},
		{"missing input", []string{"-provider", "javbus"}, true},
		{"blank input", []string{"-provider", "javbus", "-input", "  "}, true},
		{"unknown provider", []string{"-provider", "unknown", "-input", "ABC-001"}, true},
		{"unknown method", []string{"-provider", "javbus", "-input", "ABC-001", "-method", "typo"}, true},
		{"unknown flag", []string{"-typo"}, true},
		{"missing flag value", []string{"-provider"}, true},
		{"positional arguments", []string{"javbus", "ABC-001"}, true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			cmd := newCommand()
			for i := range cmd.methods {
				cmd.methods[i].call = func(context.Context, jav.Provider, string) (any, error) {
					t.Fatal("validation/help must not query a provider")
					return nil, nil
				}
			}
			var out, errOut bytes.Buffer
			err := cmd.run(context.Background(), tc.args, unexpectedReader{}, &out, &errOut)
			if (err != nil) != tc.wantError {
				t.Fatalf("error = %v", err)
			}
			if out.Len() != 0 {
				t.Fatalf("unexpected stdout: %q", out.String())
			}
			if !tc.wantError && (!strings.Contains(errOut.String(), "javdb-api") || !strings.Contains(errOut.String(), "LookupActressByJapaneseName")) {
				t.Fatalf("help missing supported options: %q", errOut.String())
			}
		})
	}
}

func TestCommandErrorsAndResultFormats(t *testing.T) {
	for _, tc := range []struct {
		name   string
		result any
		err    error
		want   string
	}{
		{"null", nil, nil, "null\n"},
		{"string", "https://example.test/movie", nil, "https://example.test/movie\n"},
		{"unsupported", nil, jav.ErrUnsupportedOperation, ""},
		{"not found", nil, jav.ErrNotFound, ""},
		{"cancelled", nil, context.Canceled, ""},
	} {
		t.Run(tc.name, func(t *testing.T) {
			cmd := newCommand()
			cmd.methods[2].call = func(context.Context, jav.Provider, string) (any, error) { return tc.result, tc.err }
			var out, errOut bytes.Buffer
			err := cmd.run(context.Background(), []string{"-provider", "javbus", "-input", "ABC-001"}, unexpectedReader{}, &out, &errOut)
			if !errors.Is(err, tc.err) || out.String() != tc.want {
				t.Fatalf("error=%v stdout=%q", err, out.String())
			}
		})
	}
}

func TestInteractiveRequiresTerminal(t *testing.T) {
	var out bytes.Buffer
	err := newCommand().run(context.Background(), nil, strings.NewReader(""), &out, io.Discard)
	if err == nil || !strings.Contains(err.Error(), "需要终端") {
		t.Fatalf("error = %v", err)
	}
}

func TestSupportedProviders(t *testing.T) {
	for _, tc := range []struct {
		method string
		want   []string
	}{
		{"LookupActressByCode", []string{"javdatabase"}},
		{"LookupActressByJapaneseName", []string{"javmodel", "minnanoav"}},
		{"LookupJavByCode", []string{"javbus", "javdatabase", "javdb", "javdb-api", "avmoo", "avsox", "javmenu", "theporndb"}},
	} {
		t.Run(tc.method, func(t *testing.T) {
			cmd := newCommand()
			method, err := cmd.findMethod(tc.method)
			if err != nil {
				t.Fatal(err)
			}
			got := providerNames(cmd.supportedProviders(method))
			if !reflect.DeepEqual(got, tc.want) {
				t.Fatalf("providers = %v, want %v", got, tc.want)
			}
		})
	}
}
