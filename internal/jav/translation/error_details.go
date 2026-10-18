package translation

import (
	"errors"
	"net/http"
	"strconv"
	"strings"
)

func ErrorDetails(err error) (int, string, string) {
	var batchErr *BatchError
	if errors.As(err, &batchErr) {
		return batchErr.Status, batchErr.ZH, batchErr.EN
	}
	var configError *ConfigError
	if errors.As(err, &configError) {
		parts := strings.SplitN(configError.Message, " / ", 2)
		en := parts[0]
		if len(parts) > 1 {
			en = parts[1]
		}
		return http.StatusBadRequest, parts[0], en
	}
	status := http.StatusBadGateway
	zh, en := "服务商请求失败或未返回完整译文，请稍后重试", "The provider failed or returned an incomplete translation; retry later"
	var apiError *APIError
	if !errors.As(err, &apiError) {
		return status, zh, en
	}
	switch apiError.StatusCode {
	case http.StatusUnauthorized, http.StatusForbidden:
		// Provider authentication failures must not expire the application's session.
		status = http.StatusBadRequest
		zh, en = "API Key 无效或无访问权限", "The API key is invalid or lacks permission"
	case http.StatusPaymentRequired:
		status = http.StatusBadRequest
		zh, en = "服务商账户余额不足", "The provider account balance is insufficient"
	case http.StatusTooManyRequests:
		status = http.StatusTooManyRequests
		zh, en = "服务商请求受到限制，请检查额度或稍后重试", "The provider restricted the request; check your quota or retry later"
	}
	// A specific provider code takes precedence over a broad error type.
	for _, reason := range []string{apiError.Code, apiError.Type} {
		matched := true
		switch reason {
		case "credit_balance_exhausted":
			zh, en = "服务商账户余额已耗尽，请充值后重试", "The provider account credit balance is exhausted; add credits and retry"
		case "organization_spend_limit_exceeded":
			zh, en = "服务商组织支出已达上限，请检查组织额度设置", "The provider organization spending limit has been reached; check its limits"
		case "project_spend_limit_exceeded":
			zh, en = "服务商项目支出已达上限，请检查项目额度设置", "The provider project spending limit has been reached; check its limits"
		case "organization_usage_limit_exceeded":
			zh, en = "服务商组织用量已达上限，请检查用量额度", "The provider organization usage limit has been reached; check its quota"
		case "insufficient_quota":
			zh, en = "服务商可用额度不足，请检查账户余额和额度限制", "The provider quota is insufficient; check your balance and limits"
		case "slow_down", "rate_limit_exceeded", "rate_limit_error":
			zh, en = "服务商请求过于频繁，请稍后重试", "Provider rate limit reached; retry later"
		default:
			matched = false
		}
		if matched {
			break
		}
	}
	diagnostic := "HTTP " + strconv.Itoa(apiError.StatusCode)
	if apiError.Code != "" {
		diagnostic += " / " + apiError.Code
	}
	if apiError.Type != "" && apiError.Type != apiError.Code {
		diagnostic += " / " + apiError.Type
	}
	diagnostic = " (" + diagnostic + ")"
	if apiError.Message != "" {
		diagnostic += ": " + apiError.Message
	}
	return status, zh + diagnostic, en + diagnostic
}
