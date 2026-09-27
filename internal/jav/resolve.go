package jav

import (
	"context"
	"errors"
	"fmt"
	"strings"
)

// ResolveJavByCodes queries the default client using the automatic provider strategy.
func ResolveJavByCodes(ctx context.Context, possibleCodes, uncensoredPossibleCodes []string) (*JavInfo, error) {
	return defaultClient.ResolveJavByCodes(ctx, possibleCodes, uncensoredPossibleCodes)
}

// ResolveJavByCodes tries candidates in order, using each code's preferred providers,
// then tries uncensoredPossibleCodes through Avsox. It returns the first hit without
// merging metadata. Empty results and ErrNotFound advance to the next lookup.
// Other lookup errors are collected and returned only if no lookup succeeds;
// ErrNotFound is returned when all lookups miss (including empty candidate lists).
// Cancellation or expiration of ctx stops resolution immediately.
func (c *Client) ResolveJavByCodes(ctx context.Context, possibleCodes, uncensoredPossibleCodes []string) (*JavInfo, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	var failures []error
	lookup := func(code string, provider Provider) (*JavInfo, error) {
		info, err := c.LookupJavByCode(ctx, code, provider)
		if ctxErr := ctx.Err(); ctxErr != nil {
			return nil, ctxErr
		}
		if err != nil {
			if !errors.Is(err, ErrNotFound) {
				failures = append(failures, fmt.Errorf("resolve jav provider=%s code=%s: %w", provider, code, err))
			}
			return nil, nil
		}
		return info, nil
	}

	for _, code := range possibleCodes {
		for _, provider := range providersForCode(code) {
			if info, err := lookup(code, provider); err != nil || info != nil {
				return info, err
			}
		}
	}
	for _, code := range uncensoredPossibleCodes {
		if info, err := lookup(code, ProviderAvsox); err != nil || info != nil {
			return info, err
		}
	}
	if len(failures) > 0 {
		return nil, errors.Join(failures...)
	}
	return nil, ErrNotFound
}

func providersForCode(code string) []Provider {
	code = strings.ToUpper(strings.TrimSpace(code))
	switch {
	case strings.HasPrefix(code, "FC2-PPV-"):
		return []Provider{ProviderJavDBAPI}
	case strings.HasPrefix(code, "GANA-"):
		return []Provider{ProviderJavMenu, ProviderJavBus}
	case strings.HasPrefix(code, "STARS-"):
		return []Provider{ProviderJavBus, ProviderAvmoo}
	case strings.HasPrefix(code, "AP-"):
		return []Provider{ProviderAvmoo}
	default:
		return []Provider{ProviderJavBus}
	}
}
