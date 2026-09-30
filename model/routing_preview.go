package model

import (
	"fmt"
	"sort"
	"strings"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/dto"
	"github.com/QuantumNous/new-api/setting/ratio_setting"
)

const RoutingPreviewStrategy = "priority_then_weighted_random"

// RoutingPreview is a read-only description of the candidates that the
// channel selector can use for a group/model pair. It never calls an upstream
// provider and deliberately contains no channel credentials.
type RoutingPreview struct {
	Group             string                `json:"group"`
	Model             string                `json:"model"`
	Strategy          string                `json:"strategy"`
	Source            string                `json:"source"`
	Tiers             []RoutingPreviewTier  `json:"tiers"`
	Excluded          []RoutingPreviewEntry `json:"excluded,omitempty"`
	UnavailableReason string                `json:"unavailable_reason,omitempty"`
}

type RoutingPreviewTier struct {
	Priority             int64                 `json:"priority"`
	TotalEffectiveWeight int                   `json:"total_effective_weight"`
	Channels             []RoutingPreviewEntry `json:"channels"`
}

type RoutingPreviewEntry struct {
	ChannelID       int     `json:"channel_id"`
	ChannelName     string  `json:"channel_name,omitempty"`
	Status          int     `json:"status,omitempty"`
	Weight          int     `json:"weight"`
	EffectiveWeight int     `json:"effective_weight"`
	Probability     float64 `json:"probability,omitempty"`
	Reason          string  `json:"reason,omitempty"`
}

type routingPreviewCandidate struct {
	channel        *Channel
	priority       int64
	weight         int
	filteredReason string
}

// GetRoutingPreview returns the same enabled candidate set used by the live
// database or memory-cache selector. The source field makes it explicit which
// path supplied the snapshot, which is useful while diagnosing cache drift.
func GetRoutingPreview(group, modelName string, filters []dto.ChannelFilter) (*RoutingPreview, error) {
	group = strings.TrimSpace(group)
	modelName = strings.TrimSpace(modelName)
	if group == "" || modelName == "" {
		return nil, fmt.Errorf("group and model are required")
	}

	candidates, source, err := routingPreviewCandidates(group, modelName, filters)
	if err != nil {
		return nil, err
	}

	preview := &RoutingPreview{
		Group:    group,
		Model:    modelName,
		Strategy: RoutingPreviewStrategy,
		Source:   source,
		Tiers:    make([]RoutingPreviewTier, 0),
		Excluded: make([]RoutingPreviewEntry, 0),
	}

	if len(candidates) == 0 {
		preview.UnavailableReason = routingPreviewUnavailableReason(group, modelName, filters)
		return preview, nil
	}

	tiers := make(map[int64][]routingPreviewCandidate)
	priorities := make([]int64, 0)
	for _, candidate := range candidates {
		if candidate.filteredReason != "" {
			preview.Excluded = append(preview.Excluded, routingPreviewEntry(candidate, candidate.filteredReason))
			continue
		}
		if _, exists := tiers[candidate.priority]; !exists {
			priorities = append(priorities, candidate.priority)
		}
		tiers[candidate.priority] = append(tiers[candidate.priority], candidate)
	}
	sort.Slice(priorities, func(i, j int) bool { return priorities[i] > priorities[j] })

	for _, priority := range priorities {
		items := tiers[priority]
		tier := RoutingPreviewTier{Priority: priority, Channels: make([]RoutingPreviewEntry, 0, len(items))}
		for _, candidate := range items {
			entry := routingPreviewEntry(candidate, "")
			// The database selector uses weight+10 for every channel. Expose the
			// same effective weight so zero-weight channels remain visible and
			// the preview explains the smoothing rule.
			tier.TotalEffectiveWeight += entry.EffectiveWeight
			tier.Channels = append(tier.Channels, entry)
		}
		if tier.TotalEffectiveWeight > 0 {
			for i := range tier.Channels {
				tier.Channels[i].Probability = float64(tier.Channels[i].EffectiveWeight) / float64(tier.TotalEffectiveWeight)
			}
		}
		preview.Tiers = append(preview.Tiers, tier)
	}
	if len(preview.Tiers) == 0 && len(preview.Excluded) > 0 {
		preview.UnavailableReason = "request_constraints_excluded_all_channels"
	}
	return preview, nil
}

func routingPreviewEntry(candidate routingPreviewCandidate, reason string) RoutingPreviewEntry {
	entry := RoutingPreviewEntry{
		ChannelID:       candidate.channel.Id,
		ChannelName:     candidate.channel.Name,
		Status:          candidate.channel.Status,
		Weight:          candidate.weight,
		EffectiveWeight: candidate.weight + 10,
		Reason:          reason,
	}
	return entry
}

func routingPreviewCandidates(group, modelName string, filters []dto.ChannelFilter) ([]routingPreviewCandidate, string, error) {
	if common.MemoryCacheEnabled {
		candidates, ok := routingPreviewCacheCandidates(group, modelName, filters)
		if ok {
			return candidates, "memory_cache", nil
		}
	}
	return routingPreviewDatabaseCandidates(group, modelName, filters)
}

func routingPreviewCacheCandidates(group, modelName string, filters []dto.ChannelFilter) ([]routingPreviewCandidate, bool) {
	channelSyncLock.RLock()
	defer channelSyncLock.RUnlock()
	if group2model2channels == nil || channelsIDM == nil {
		return nil, false
	}
	ids := group2model2channels[group][modelName]
	if len(ids) == 0 {
		normalized := ratio_setting.RoutingMatchModelName(modelName)
		ids = group2model2channels[group][normalized]
	}
	if len(ids) == 0 {
		return nil, true
	}
	result := make([]routingPreviewCandidate, 0, len(ids))
	for _, id := range ids {
		channel, exists := channelsIDM[id]
		if !exists || channel == nil {
			continue
		}
		candidate := routingPreviewCandidate{channel: channel, priority: channel.GetPriority(), weight: channel.GetWeight()}
		if ok, kind := ChannelSatisfiesFilters(channel, modelName, filters); !ok {
			candidate.filteredReason = string(kind)
		}
		result = append(result, candidate)
	}
	return result, true
}

func routingPreviewDatabaseCandidates(group, modelName string, filters []dto.ChannelFilter) ([]routingPreviewCandidate, string, error) {
	var abilities []Ability
	if err := DB.Where(commonGroupCol+" = ? and model = ? and enabled = ?", group, modelName, true).
		Order("priority DESC, weight DESC, channel_id ASC").Find(&abilities).Error; err != nil {
		return nil, "database", err
	}
	if len(abilities) == 0 {
		return []routingPreviewCandidate{}, "database", nil
	}

	ids := make([]int, 0, len(abilities))
	for _, ability := range abilities {
		ids = append(ids, ability.ChannelId)
	}
	var channels []*Channel
	if err := DB.Where("id IN ?", ids).Find(&channels).Error; err != nil {
		return nil, "database", err
	}
	channelsByID := make(map[int]*Channel, len(channels))
	for _, channel := range channels {
		channelsByID[channel.Id] = channel
	}

	result := make([]routingPreviewCandidate, 0, len(abilities))
	for _, ability := range abilities {
		channel := channelsByID[ability.ChannelId]
		if channel == nil {
			continue
		}
		candidate := routingPreviewCandidate{
			channel:  channel,
			priority: abilityPriority(ability),
			weight:   int(ability.Weight),
		}
		if ok, kind := ChannelSatisfiesFilters(channel, modelName, filters); !ok {
			candidate.filteredReason = string(kind)
		}
		result = append(result, candidate)
	}
	return result, "database", nil
}

func abilityPriority(ability Ability) int64 {
	if ability.Priority == nil {
		return 0
	}
	return *ability.Priority
}

func routingPreviewUnavailableReason(group, modelName string, filters []dto.ChannelFilter) string {
	var count int64
	DB.Model(&Ability{}).Where(commonGroupCol+" = ? and enabled = ?", group, true).Count(&count)
	if count == 0 {
		return "no_enabled_channel_for_group"
	}
	var modelCount int64
	modelQuery := DB.Model(&Ability{}).Where(commonGroupCol+" = ? and model = ? and enabled = ?", group, modelName, true)
	modelQuery.Count(&modelCount)
	if modelCount == 0 {
		return "model_not_supported"
	}
	if len(filters) > 0 {
		return "request_constraints_excluded_all_channels"
	}
	return "no_available_channel"
}
