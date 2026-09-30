package controller

import (
	"net/http"
	"strconv"
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestGroupChainCRUDAndTokenCount(t *testing.T) {
	configureTokenAutoGroupsTest(t, "5", `["default","vip"]`)
	user := setupTokenAutoGroupsControllerTest(t)
	require.NoError(t, model.DB.AutoMigrate(&model.Token{}))

	createCtx, createRecorder := newTokenAutoGroupsAuthenticatedContext(
		t, http.MethodPost, "/api/user/group_chains", map[string]any{
			"name":   "Primary fallback",
			"groups": []string{"vip", "default"},
		}, user.Id,
	)
	CreateGroupChain(createCtx)
	createResponse := decodeAPIResponse(t, createRecorder)
	require.True(t, createResponse.Success, createResponse.Message)
	var created groupChainResponse
	require.NoError(t, common.Unmarshal(createResponse.Data, &created))
	assert.Equal(t, []string{"vip", "default"}, created.Groups)
	assert.Zero(t, created.TokenCount)

	token := seedToken(t, model.DB, user.Id, "chain-token", "chain-token-key")
	token.Group = model.GroupChainValue(created.Id)
	require.NoError(t, model.DB.Save(token).Error)

	getCtx, getRecorder := newTokenAutoGroupsAuthenticatedContext(
		t, http.MethodGet, "/api/user/group_chains", nil, user.Id,
	)
	GetGroupChains(getCtx)
	getResponse := decodeAPIResponse(t, getRecorder)
	require.True(t, getResponse.Success, getResponse.Message)
	var listing struct {
		Items []groupChainResponse `json:"items"`
	}
	require.NoError(t, common.Unmarshal(getResponse.Data, &listing))
	require.Len(t, listing.Items, 1)
	assert.Equal(t, int64(1), listing.Items[0].TokenCount)

	updateCtx, updateRecorder := newTokenAutoGroupsAuthenticatedContext(
		t, http.MethodPut, "/api/user/group_chains/"+strconv.Itoa(created.Id), map[string]any{
			"name":   "Updated fallback",
			"groups": []string{"default", "vip"},
		}, user.Id,
	)
	updateCtx.Params = append(updateCtx.Params, gin.Param{Key: "id", Value: strconv.Itoa(created.Id)})
	UpdateGroupChain(updateCtx)
	updateResponse := decodeAPIResponse(t, updateRecorder)
	require.True(t, updateResponse.Success, updateResponse.Message)
	var updated groupChainResponse
	require.NoError(t, common.Unmarshal(updateResponse.Data, &updated))
	assert.Equal(t, "Updated fallback", updated.Name)
	assert.Equal(t, []string{"default", "vip"}, updated.Groups)
	assert.Equal(t, int64(1), updated.TokenCount)

	deleteCtx, deleteRecorder := newTokenAutoGroupsAuthenticatedContext(
		t, http.MethodDelete, "/api/user/group_chains/"+strconv.Itoa(created.Id), nil, user.Id,
	)
	deleteCtx.Params = append(deleteCtx.Params, gin.Param{Key: "id", Value: strconv.Itoa(created.Id)})
	DeleteGroupChain(deleteCtx)
	deleteResponse := decodeAPIResponse(t, deleteRecorder)
	require.True(t, deleteResponse.Success, deleteResponse.Message)
	var migrated model.Token
	require.NoError(t, model.DB.First(&migrated, token.Id).Error)
	assert.Equal(t, "auto", migrated.Group)
	assert.True(t, migrated.CrossGroupRetry)
}
