package service

import (
	"net/http"

	"github.com/QuantumNous/new-api/relaykit/types"
	"github.com/QuantumNous/new-api/setting/operation_setting"
)

// RetryClassification is the stable category written to request policy
// events. The action (retry/stop) remains separate so callers can explain a
// cooldown retry without treating it as a permanent failure.
type RetryClassification string

const (
	RetryClassificationRetryable    RetryClassification = "retryable"
	RetryClassificationNonRetryable RetryClassification = "non_retryable"
	RetryClassificationCooldown     RetryClassification = "cooldown"
	RetryClassificationManual       RetryClassification = "manual"
)

// ClassifyRelayError applies the shared HTTP and NewAPI error rules used by
// HTTP, streaming, and Responses WebSocket relay paths.
func ClassifyRelayError(err *types.NewAPIError) RetryClassification {
	if err == nil {
		return RetryClassificationNonRetryable
	}
	if types.IsChannelError(err) {
		return RetryClassificationRetryable
	}
	if types.IsSkipRetryError(err) {
		return RetryClassificationNonRetryable
	}
	return ClassifyRetryStatus(err.StatusCode, false, false)
}

// ClassifyRetryStatus is shared by TaskError and NewAPIError adapters so all
// relay protocols classify the same upstream status consistently.
func ClassifyRetryStatus(status int, localError bool, noRetry bool) RetryClassification {
	if noRetry || localError {
		return RetryClassificationManual
	}
	switch status {
	case http.StatusUnauthorized, http.StatusForbidden, http.StatusPaymentRequired,
		http.StatusBadRequest, http.StatusNotFound, http.StatusRequestTimeout,
		http.StatusConflict, http.StatusGone, http.StatusUnprocessableEntity:
		return RetryClassificationManual
	case http.StatusTooManyRequests, http.StatusInternalServerError, http.StatusBadGateway,
		http.StatusServiceUnavailable:
		if !operation_setting.IsAlwaysSkipRetryStatusCode(status) && operation_setting.ShouldRetryByStatusCode(status) {
			return RetryClassificationCooldown
		}
	}
	if status >= 100 && status < 600 && operation_setting.ShouldRetryByStatusCode(status) {
		return RetryClassificationRetryable
	}
	return RetryClassificationNonRetryable
}
