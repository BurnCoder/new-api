package model

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestNormalizeGroupChainGroups(t *testing.T) {
	tests := []struct {
		name    string
		input   []string
		want    []string
		wantErr string
	}{
		{name: "trims while preserving order", input: []string{" vip ", "default"}, want: []string{"vip", "default"}},
		{name: "rejects empty chain", input: nil, wantErr: "至少需要一个分组"},
		{name: "rejects auto", input: []string{"auto"}, wantErr: "不能包含 auto"},
		{name: "rejects duplicate", input: []string{"vip", "vip"}, wantErr: "重复分组"},
		{name: "rejects empty member", input: []string{"vip", " "}, wantErr: "名称不能为空"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := NormalizeGroupChainGroups(tt.input)
			if tt.wantErr != "" {
				require.ErrorContains(t, err, tt.wantErr)
				return
			}
			require.NoError(t, err)
			require.Equal(t, tt.want, got)
		})
	}
}

func TestGroupChainSetAndGetGroups(t *testing.T) {
	chain := &GroupChain{}
	require.NoError(t, chain.SetGroups([]string{"vip", "default"}))
	groups, err := chain.GetGroups()
	require.NoError(t, err)
	require.Equal(t, []string{"vip", "default"}, groups)
}

func TestGroupChainValueParsingIsStrict(t *testing.T) {
	validID, valid := ParseGroupChainValue(GroupChainValue(42))
	require.True(t, valid)
	require.Equal(t, 42, validID)

	for _, value := range []string{"chain:", "chain:0", "chain:-1", "chain:42x", "group:42"} {
		_, ok := ParseGroupChainValue(value)
		require.False(t, ok, value)
	}
}
