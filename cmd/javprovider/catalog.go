package main

import (
	"context"
	"fmt"
	"strings"

	"javboss/internal/jav"
)

type methodOption struct {
	supports func(jav.Capabilities) bool
	name     string
	prompt   string
	call     func(context.Context, jav.Provider, string) (any, error)
}

type providerOption struct {
	name     string
	provider jav.Provider
}

type command struct {
	providers []providerOption
	methods   []methodOption
}

func newCommand() command {
	providers := make([]providerOption, 0, 11)
	for _, provider := range []jav.Provider{
		jav.ProviderJavBus, jav.ProviderJavDatabase, jav.ProviderJavDB, jav.ProviderJavDBAPI,
		jav.ProviderAvmoo, jav.ProviderAvsox, jav.ProviderJavMenu, jav.ProviderJavModel,
		jav.ProviderThePornDB, jav.ProviderMinnanoAV, jav.ProviderAVWiki,
	} {
		providers = append(providers, providerOption{name: provider.String(), provider: provider})
	}

	methods := []methodOption{
		{
			name:     "LookupActressByCode",
			supports: func(c jav.Capabilities) bool { return c.ActressByCode },
			prompt:   "请输入番号",
			call: func(ctx context.Context, provider jav.Provider, input string) (any, error) {
				return jav.LookupActressByCode(ctx, input, provider)
			},
		},
		{
			name:     "LookupActressByJapaneseName",
			supports: func(c jav.Capabilities) bool { return c.ActressByName },
			prompt:   "请输入女优日文名",
			call: func(ctx context.Context, provider jav.Provider, input string) (any, error) {
				return jav.LookupActressByJapaneseName(ctx, input, provider)
			},
		},
		{
			name:     "LookupJavByCode",
			supports: func(c jav.Capabilities) bool { return c.Movie },
			prompt:   "请输入番号",
			call: func(ctx context.Context, provider jav.Provider, input string) (any, error) {
				return jav.LookupJavByCode(ctx, input, provider)
			},
		},
	}

	return command{providers: providers, methods: methods}
}

func providerNames(providers []providerOption) []string {
	names := make([]string, 0, len(providers))
	for _, provider := range providers {
		names = append(names, provider.name)
	}
	return names
}

func methodNames(methods []methodOption) []string {
	names := make([]string, 0, len(methods))
	for _, method := range methods {
		names = append(names, method.name)
	}
	return names
}

func (cmd command) supportedProviders(method methodOption) []providerOption {
	providers := make([]providerOption, 0, len(cmd.providers))
	for _, provider := range cmd.providers {
		if method.supports(jav.CapabilitiesFor(provider.provider)) {
			providers = append(providers, provider)
		}
	}
	return providers
}

func (cmd command) findProvider(name string) (providerOption, error) {
	for _, provider := range cmd.providers {
		if strings.EqualFold(provider.name, strings.TrimSpace(name)) {
			return provider, nil
		}
	}
	return providerOption{}, fmt.Errorf("未知 provider %q，可选: %s", name, strings.Join(providerNames(cmd.providers), ", "))
}

func (cmd command) findMethod(name string) (methodOption, error) {
	for _, method := range cmd.methods {
		if strings.EqualFold(method.name, strings.TrimSpace(name)) {
			return method, nil
		}
	}
	return methodOption{}, fmt.Errorf("未知 method %q，可选: %s", name, strings.Join(methodNames(cmd.methods), ", "))
}
