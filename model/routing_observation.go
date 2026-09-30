package model

import (
	"errors"
	"sort"
	"strings"

	"github.com/QuantumNous/new-api/common"
)

// RoutingObservationQuery describes the admin-only filters for the routing
// observability endpoint. Credential material is never accepted as a filter;
// key_fp is a one-way fingerprint generated during request authentication.
type RoutingObservationQuery struct {
	StartTimestamp int64
	EndTimestamp   int64
	ModelName      string
	TokenName      string
	KeyFingerprint string
	ChannelID      int
	Group          string
	Classification string
	Result         string
}

type RoutingObservationItem struct {
	LogID        int              `json:"log_id"`
	CreatedAt    int64            `json:"created_at"`
	RequestID    string           `json:"request_id"`
	UserID       int              `json:"user_id"`
	Username     string           `json:"username"`
	TokenName    string           `json:"token_name"`
	ModelName    string           `json:"model_name"`
	Group        string           `json:"group"`
	ChannelID    int              `json:"channel_id"`
	ChannelName  string           `json:"channel_name,omitempty"`
	Observation  map[string]any   `json:"observation"`
	PolicyEvents []map[string]any `json:"policy_events,omitempty"`
}

type RoutingObservationResult struct {
	Items []RoutingObservationItem `json:"items"`
	Total int64                    `json:"total"`
}

type RoutingHealthStat struct {
	ChannelID           int     `json:"channel_id"`
	Attempts            int64   `json:"attempts"`
	Successes           int64   `json:"successes"`
	Failures            int64   `json:"failures"`
	SuccessRate         float64 `json:"success_rate"`
	AverageLatencyMS    float64 `json:"average_latency_ms"`
	ConsecutiveFailures int64   `json:"consecutive_failures"`
	CooldownUntil       int64   `json:"cooldown_until"`
	LastSwitchAt        int64   `json:"last_switch_at"`
}

type routingHealthAccumulator struct {
	RoutingHealthStat
	latencyTotal int64
	latencyCount int64
}

func routingObservationMap(other string) (map[string]any, []map[string]any, bool) {
	var payload map[string]any
	if other == "" || common.Unmarshal([]byte(other), &payload) != nil {
		return nil, nil, false
	}
	adminInfo, ok := payload["admin_info"].(map[string]any)
	if !ok {
		return nil, nil, false
	}
	observation, ok := adminInfo["route_observation"].(map[string]any)
	if !ok {
		return nil, nil, false
	}
	var events []map[string]any
	if rawEvents, ok := adminInfo["request_policy"].([]any); ok {
		for _, raw := range rawEvents {
			if event, ok := raw.(map[string]any); ok {
				events = append(events, event)
			}
		}
	}
	return observation, events, true
}

func routingObservationMatches(observation map[string]any, query RoutingObservationQuery) bool {
	if query.KeyFingerprint != "" && stringValue(observation["key_fp"]) != query.KeyFingerprint {
		return false
	}
	if query.Result != "" && stringValue(observation["result"]) != query.Result {
		return false
	}
	if query.Classification != "" && stringValue(observation["classification"]) != query.Classification {
		return false
	}
	if query.ChannelID != 0 {
		if intValue(observation["final_channel_id"]) == query.ChannelID {
			return true
		}
		for _, raw := range numberSlice(observation["candidate_channel_ids"]) {
			if raw == query.ChannelID {
				return true
			}
		}
		return false
	}
	return true
}

func stringValue(value any) string {
	text, _ := value.(string)
	return strings.TrimSpace(text)
}

func intValue(value any) int {
	switch number := value.(type) {
	case float64:
		return int(number)
	case int:
		return number
	case int64:
		return int(number)
	default:
		return 0
	}
}

func numberSlice(value any) []int {
	values, ok := value.([]any)
	if !ok {
		return nil
	}
	result := make([]int, 0, len(values))
	for _, value := range values {
		result = append(result, intValue(value))
	}
	return result
}

// GetRoutingObservations reads the structured route metadata embedded in usage
// logs. Existing log filters run in SQL first; the small admin-only fields that
// live in JSON are then matched in memory, keeping the schema backwards
// compatible for SQLite, MySQL, PostgreSQL and ClickHouse.
func GetRoutingObservations(query RoutingObservationQuery, startIdx, limit int) (RoutingObservationResult, error) {
	if LOG_DB == nil {
		return RoutingObservationResult{}, errors.New("log database is not initialized")
	}
	if limit <= 0 {
		limit = 50
	}
	if limit > logSearchCountLimit {
		limit = logSearchCountLimit
	}
	tx := LOG_DB.Model(&Log{})
	if query.StartTimestamp != 0 {
		tx = tx.Where("created_at >= ?", query.StartTimestamp)
	}
	if query.EndTimestamp != 0 {
		tx = tx.Where("created_at <= ?", query.EndTimestamp)
	}
	if query.ModelName != "" {
		tx = tx.Where("model_name = ?", query.ModelName)
	}
	if query.TokenName != "" {
		tx = tx.Where("token_name = ?", query.TokenName)
	}
	if query.Group != "" {
		tx = tx.Where(logGroupCol+" = ?", query.Group)
	}
	var logs []*Log
	if err := tx.Order("created_at desc, id desc").Limit(logSearchCountLimit).Find(&logs).Error; err != nil {
		return RoutingObservationResult{}, err
	}
	filtered := make([]RoutingObservationItem, 0, len(logs))
	for _, log := range logs {
		observation, events, ok := routingObservationMap(log.Other)
		if !ok || !routingObservationMatches(observation, query) {
			continue
		}
		filtered = append(filtered, RoutingObservationItem{
			LogID: log.Id, CreatedAt: log.CreatedAt, RequestID: log.RequestId,
			UserID: log.UserId, Username: log.Username, TokenName: log.TokenName,
			ModelName: log.ModelName, Group: log.Group, ChannelID: log.ChannelId,
			Observation: observation, PolicyEvents: events,
		})
	}
	result := RoutingObservationResult{Total: int64(len(filtered))}
	if startIdx < 0 {
		startIdx = 0
	}
	if startIdx >= len(filtered) {
		return result, nil
	}
	end := startIdx + limit
	if end > len(filtered) {
		end = len(filtered)
	}
	result.Items = filtered[startIdx:end]
	return result, nil
}

// SummarizeRoutingHealth derives operational health from the same structured
// events used by the request detail view. It is intentionally read-only and
// bounded by the caller's time window, so request handling never waits on a
// second health-write transaction.
func SummarizeRoutingHealth(items []RoutingObservationItem, now int64) []RoutingHealthStat {
	acc := make(map[int]*routingHealthAccumulator)
	for i := len(items) - 1; i >= 0; i-- {
		item := items[i]
		attemptStart := make(map[int]int64)
		for _, event := range item.PolicyEvents {
			channelID := intValue(event["channel_id"])
			if channelID == 0 {
				continue
			}
			decision, _ := event["decision"].(map[string]any)
			action := stringValue(decision["action"])
			state := acc[channelID]
			if state == nil {
				state = &routingHealthAccumulator{RoutingHealthStat: RoutingHealthStat{ChannelID: channelID}}
				acc[channelID] = state
			}
			switch action {
			case "attempt":
				state.Attempts++
				attemptStart[channelID] = intValue64(event["elapsed_ms"])
			case "failure":
				state.Failures++
				state.ConsecutiveFailures++
				state.RoutingHealthStat.CooldownUntil = 0
				if stringValue(event["classification"]) == "cooldown" {
					state.CooldownUntil = item.CreatedAt + 60
				}
				if started, ok := attemptStart[channelID]; ok {
					state.latencyTotal += intValue64(event["elapsed_ms"]) - started
					state.latencyCount++
				}
			case "success":
				state.Successes++
				state.ConsecutiveFailures = 0
				if started, ok := attemptStart[channelID]; ok {
					state.latencyTotal += intValue64(event["elapsed_ms"]) - started
					state.latencyCount++
				}
			}
		}
		if intValue(item.Observation["switch_count"]) > 0 {
			for _, channelID := range numberSlice(item.Observation["candidate_channel_ids"]) {
				if state := acc[channelID]; state != nil {
					state.LastSwitchAt = item.CreatedAt
				}
			}
		}
	}
	result := make([]RoutingHealthStat, 0, len(acc))
	for _, state := range acc {
		if state.Attempts > 0 {
			state.SuccessRate = float64(state.Successes) / float64(state.Attempts)
		}
		if state.latencyCount > 0 {
			state.AverageLatencyMS = float64(state.latencyTotal) / float64(state.latencyCount)
		}
		if state.CooldownUntil <= now {
			state.CooldownUntil = 0
		}
		result = append(result, state.RoutingHealthStat)
	}
	sort.Slice(result, func(i, j int) bool { return result[i].ChannelID < result[j].ChannelID })
	return result
}

func intValue64(value any) int64 {
	switch number := value.(type) {
	case float64:
		return int64(number)
	case int:
		return int64(number)
	case int64:
		return number
	default:
		return 0
	}
}
