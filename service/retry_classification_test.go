package service

import (
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/QuantumNous/new-api/dto"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/relaykit/types"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestClassifyRelayError(t *testing.T) {
	tests := []struct {
		name      string
		status    int
		code      types.ErrorCode
		want      RetryClassification
		skipRetry bool
	}{
		{name: "rate limit cools down", status: http.StatusTooManyRequests, want: RetryClassificationCooldown},
		{name: "temporary upstream failure", status: http.StatusBadGateway, want: RetryClassificationCooldown},
		{name: "credential requires manual action", status: http.StatusUnauthorized, want: RetryClassificationManual},
		{name: "invalid request requires manual action", status: http.StatusBadRequest, want: RetryClassificationManual},
		{name: "successful response is not retryable", status: http.StatusOK, want: RetryClassificationNonRetryable},
		{name: "explicit skip retry", status: http.StatusInternalServerError, code: types.ErrorCodeInvalidRequest, want: RetryClassificationNonRetryable, skipRetry: true},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			opts := []types.NewAPIErrorOptions{}
			if test.skipRetry {
				opts = append(opts, types.ErrOptionWithSkipRetry())
			}
			err := types.NewErrorWithStatusCode(errors.New(test.name), test.code, test.status, opts...)
			assert.Equal(t, test.want, ClassifyRelayError(err))
		})
	}
}

func TestRetryParamSelectionFiltersFailedChannels(t *testing.T) {
	param := &RetryParam{FailedChannels: map[int]struct{}{9: {}, 3: {}}}
	filters := param.selectionFilters(nil)
	require.Len(t, filters, 1)
	assert.Equal(t, dto.FilterExcludedChannels, filters[0].Kind)
	assert.Equal(t, []int{3, 9}, filters[0].ExcludedChannelIDs)
}

func TestRequestPolicyRecordsRetryClassificationAndPreviousChannel(t *testing.T) {
	c, _ := gin.CreateTestContext(httptest.NewRecorder())
	state := RequestPolicy(c)
	state.BeginAttempt(&model.Channel{Id: 41}, "default")
	err := types.NewOpenAIError(errors.New("rate limited"), types.ErrorCodeBadResponseStatusCode, http.StatusTooManyRequests)
	decision := DecideRelayRetry(c, err, 1)
	RecordPolicyFailure(c, 41, err, decision)
	state.BeginAttempt(&model.Channel{Id: 42}, "default")

	events := state.Events()
	require.Len(t, events, 4)
	assert.Equal(t, 0, events[0].RetryIndex)
	assert.Equal(t, RetryClassificationCooldown, RetryClassification(events[1].Classification))
	assert.Equal(t, 41, events[1].PreviousChannelID)
	assert.Equal(t, 1, events[3].RetryIndex)
	assert.Equal(t, 41, events[3].PreviousChannelID)
}
