package model

import (
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/glebarez/sqlite"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

func setupRoutingPreviewTest(t *testing.T) {
	t.Helper()
	previousDB := DB
	previousCache := common.MemoryCacheEnabled
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(&Channel{}, &Ability{}))
	DB = db
	common.MemoryCacheEnabled = false
	t.Cleanup(func() {
		DB = previousDB
		common.MemoryCacheEnabled = previousCache
	})
}

func TestGetRoutingPreviewGroupsByPriorityAndUsesEffectiveWeight(t *testing.T) {
	setupRoutingPreviewTest(t)
	channels := []*Channel{
		{Id: 1001, Name: "primary", Status: common.ChannelStatusEnabled, Models: "gpt-test", Group: "default", Weight: uintPtr(80), Priority: int64Ptr(10)},
		{Id: 1002, Name: "secondary", Status: common.ChannelStatusEnabled, Models: "gpt-test", Group: "default", Weight: uintPtr(20), Priority: int64Ptr(10)},
		{Id: 1003, Name: "fallback", Status: common.ChannelStatusEnabled, Models: "gpt-test", Group: "default", Weight: uintPtr(0), Priority: int64Ptr(5)},
	}
	for _, channel := range channels {
		require.NoError(t, DB.Create(channel).Error)
		require.NoError(t, DB.Create(&Ability{
			Group: channel.Group, Model: "gpt-test", ChannelId: channel.Id,
			Enabled: true, Priority: channel.Priority, Weight: *channel.Weight,
		}).Error)
	}

	preview, err := GetRoutingPreview("default", "gpt-test", nil)
	require.NoError(t, err)
	assert.Equal(t, "database", preview.Source)
	assert.Equal(t, RoutingPreviewStrategy, preview.Strategy)
	require.Len(t, preview.Tiers, 2)
	assert.Equal(t, int64(10), preview.Tiers[0].Priority)
	assert.Equal(t, 120, preview.Tiers[0].TotalEffectiveWeight)
	require.Len(t, preview.Tiers[0].Channels, 2)
	assert.Equal(t, 90, preview.Tiers[0].Channels[0].EffectiveWeight)
	assert.Equal(t, 30, preview.Tiers[0].Channels[1].EffectiveWeight)
	assert.InDelta(t, 0.75, preview.Tiers[0].Channels[0].Probability, 0.0001)
	assert.Equal(t, int64(5), preview.Tiers[1].Priority)
	assert.Equal(t, 10, preview.Tiers[1].Channels[0].EffectiveWeight)
}

func TestGetRoutingPreviewExplainsUnavailableModel(t *testing.T) {
	setupRoutingPreviewTest(t)
	channel := &Channel{Id: 1004, Name: "chat", Status: common.ChannelStatusEnabled, Models: "gpt-test", Group: "default"}
	require.NoError(t, DB.Create(channel).Error)
	require.NoError(t, DB.Create(&Ability{
		Group: "default", Model: "gpt-test", ChannelId: channel.Id, Enabled: true,
	}).Error)

	preview, err := GetRoutingPreview("default", "missing-model", nil)
	require.NoError(t, err)
	assert.Empty(t, preview.Tiers)
	assert.Equal(t, "model_not_supported", preview.UnavailableReason)
}

func uintPtr(value uint) *uint { return &value }

func int64Ptr(value int64) *int64 { return &value }
