package controller

import (
	"fmt"
	"net/http"
	"strconv"
	"strings"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/service"
	"github.com/QuantumNous/new-api/setting/ratio_setting"
	"github.com/gin-gonic/gin"
)

type groupChainRequest struct {
	Name   string   `json:"name"`
	Groups []string `json:"groups"`
}

type groupChainResponse struct {
	Id          int      `json:"id"`
	UserId      int      `json:"user_id"`
	Name        string   `json:"name"`
	Groups      []string `json:"groups"`
	CreatedTime int64    `json:"created_time"`
	UpdatedTime int64    `json:"updated_time"`
}

func toGroupChainResponse(chain *model.GroupChain) (*groupChainResponse, error) {
	groups, err := chain.GetGroups()
	if err != nil {
		return nil, err
	}
	return &groupChainResponse{
		Id: chain.Id, UserId: chain.UserId, Name: chain.Name, Groups: groups,
		CreatedTime: chain.CreatedTime, UpdatedTime: chain.UpdatedTime,
	}, nil
}

func getGroupChainUserGroup(c *gin.Context) (string, error) {
	if group := c.GetString("group"); group != "" {
		return group, nil
	}
	return model.GetUserGroup(c.GetInt("id"), false)
}

func validateGroupChainRequest(c *gin.Context, request *groupChainRequest) ([]string, error) {
	request.Name = strings.TrimSpace(request.Name)
	if request.Name == "" {
		return nil, fmt.Errorf("链名称不能为空")
	}
	if len([]rune(request.Name)) > 100 {
		return nil, fmt.Errorf("链名称不能超过 100 个字符")
	}
	groups, err := model.NormalizeGroupChainGroups(request.Groups)
	if err != nil {
		return nil, err
	}
	userGroup, err := getGroupChainUserGroup(c)
	if err != nil {
		return nil, err
	}
	usable := service.GetUserUsableGroups(userGroup)
	configured := ratio_setting.GetGroupRatioCopy()
	for _, group := range groups {
		if _, ok := usable[group]; !ok {
			return nil, fmt.Errorf("无权访问 %s 分组", group)
		}
		if _, ok := configured[group]; !ok {
			return nil, fmt.Errorf("分组 %s 已被弃用", group)
		}
	}
	return groups, nil
}

func GetGroupChains(c *gin.Context) {
	chains, err := model.GetUserGroupChains(c.GetInt("id"))
	if err != nil {
		common.ApiError(c, err)
		return
	}
	items := make([]*groupChainResponse, 0, len(chains))
	for _, chain := range chains {
		item, err := toGroupChainResponse(chain)
		if err != nil {
			common.ApiError(c, err)
			return
		}
		items = append(items, item)
	}
	common.ApiSuccess(c, gin.H{
		"items": items,
		"total": len(items),
		"limit": model.GroupChainMaxPerUser,
	})
}

func CreateGroupChain(c *gin.Context) {
	var request groupChainRequest
	if err := c.ShouldBindJSON(&request); err != nil {
		common.ApiError(c, err)
		return
	}
	groups, err := validateGroupChainRequest(c, &request)
	if err != nil {
		common.ApiErrorMsg(c, err.Error())
		return
	}
	userID := c.GetInt("id")
	count, err := model.CountUserGroupChains(userID)
	if err != nil {
		common.ApiError(c, err)
		return
	}
	if count >= model.GroupChainMaxPerUser {
		common.ApiErrorMsg(c, fmt.Sprintf("最多创建 %d 条分组链", model.GroupChainMaxPerUser))
		return
	}
	if duplicated, err := model.IsGroupChainNameDuplicated(userID, 0, request.Name); err != nil {
		common.ApiError(c, err)
		return
	} else if duplicated {
		common.ApiErrorMsg(c, "分组链名称已存在")
		return
	}
	chain := &model.GroupChain{UserId: userID, Name: request.Name}
	if err := chain.SetGroups(groups); err != nil {
		common.ApiError(c, err)
		return
	}
	if err := chain.Insert(); err != nil {
		common.ApiError(c, err)
		return
	}
	response, err := toGroupChainResponse(chain)
	if err != nil {
		common.ApiError(c, err)
		return
	}
	common.ApiSuccess(c, response)
}

func UpdateGroupChain(c *gin.Context) {
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		common.ApiError(c, err)
		return
	}
	var request groupChainRequest
	if err := c.ShouldBindJSON(&request); err != nil {
		common.ApiError(c, err)
		return
	}
	groups, err := validateGroupChainRequest(c, &request)
	if err != nil {
		common.ApiErrorMsg(c, err.Error())
		return
	}
	userID := c.GetInt("id")
	chain, err := model.GetUserGroupChain(id, userID)
	if err != nil {
		common.ApiError(c, err)
		return
	}
	if duplicated, err := model.IsGroupChainNameDuplicated(userID, id, request.Name); err != nil {
		common.ApiError(c, err)
		return
	} else if duplicated {
		common.ApiErrorMsg(c, "分组链名称已存在")
		return
	}
	chain.Name = request.Name
	if err := chain.SetGroups(groups); err != nil {
		common.ApiError(c, err)
		return
	}
	if err := chain.Update(); err != nil {
		common.ApiError(c, err)
		return
	}
	response, err := toGroupChainResponse(chain)
	if err != nil {
		common.ApiError(c, err)
		return
	}
	common.ApiSuccess(c, response)
}

func DeleteGroupChain(c *gin.Context) {
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		common.ApiError(c, err)
		return
	}
	if err := model.DeleteUserGroupChain(id, c.GetInt("id")); err != nil {
		common.ApiError(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"success": true, "message": ""})
}
