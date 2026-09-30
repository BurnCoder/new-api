package model

import (
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/glebarez/sqlite"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

func TestGetRoutingObservationsFiltersRedactedMetadata(t *testing.T) {
	previous := LOG_DB
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(&Log{}))
	LOG_DB = db
	t.Cleanup(func() {
		LOG_DB = previous
	})

	other := common.MapToJsonStr(map[string]any{
		"admin_info": map[string]any{
			"route_observation": map[string]any{
				"request_id":            "req-route-1",
				"key_fp":                AccessTokenFingerprint("token-secret"),
				"model":                 "gpt-route",
				"group":                 "default",
				"candidate_channel_ids": []int{11, 12},
				"final_channel_id":      12,
				"retry_index":           1,
				"switch_count":          1,
				"result":                "success",
				"classification":        "cooldown",
			},
			"request_policy": []map[string]any{{
				"channel_id": 11,
				"elapsed_ms": 10,
				"decision":   map[string]any{"action": "attempt"},
			}, {
				"channel_id":     11,
				"elapsed_ms":     60,
				"classification": "cooldown",
				"decision":       map[string]any{"action": "failure"},
			}, {
				"channel_id": 12,
				"elapsed_ms": 70,
				"decision":   map[string]any{"action": "attempt"},
			}, {
				"channel_id": 12,
				"elapsed_ms": 100,
				"decision":   map[string]any{"action": "success"},
			}},
		},
	})
	require.NoError(t, db.Create(&Log{Id: 1, CreatedAt: 100, RequestId: "req-route-1", UserId: 9, Username: "owner", TokenName: "key", ModelName: "gpt-route", Group: "default", ChannelId: 12, Other: other}).Error)

	result, err := GetRoutingObservations(RoutingObservationQuery{KeyFingerprint: AccessTokenFingerprint("token-secret"), Classification: "cooldown", ChannelID: 11}, 0, 20)
	require.NoError(t, err)
	require.Len(t, result.Items, 1)
	require.Equal(t, "req-route-1", result.Items[0].RequestID)
	stats := SummarizeRoutingHealth(result.Items, 200)
	require.Len(t, stats, 2)
	require.Equal(t, int64(1), stats[0].Attempts)
	require.Equal(t, int64(1), stats[0].Failures)
	require.Equal(t, int64(1), stats[1].Successes)
	require.NotContains(t, other, "token-secret")
}
