package controller

import (
	"strconv"
	"strings"

	"github.com/QuantumNous/new-api/common"
	channelDto "github.com/QuantumNous/new-api/dto"
	"github.com/QuantumNous/new-api/model"

	"github.com/gin-gonic/gin"
)

// GetRoutingPreview exposes the channel selector's read-only candidate
// snapshot. It does not perform an upstream request and never returns keys or
// proxy credentials.
func GetRoutingPreview(c *gin.Context) {
	group := strings.TrimSpace(c.Query("group"))
	modelName := strings.TrimSpace(c.Query("model"))
	if group == "" || modelName == "" {
		common.ApiErrorMsg(c, "group and model are required")
		return
	}

	filters := make([]channelDto.ChannelFilter, 0, 2)
	if requestPath := strings.TrimSpace(c.Query("request_path")); requestPath != "" {
		filters = append(filters, channelDto.ChannelFilter{
			Kind:        channelDto.FilterRequestPath,
			RequestPath: requestPath,
		})
	}
	if websocket := strings.TrimSpace(c.Query("responses_websocket")); websocket != "" {
		enabled, err := strconv.ParseBool(websocket)
		if err != nil {
			common.ApiErrorMsg(c, "responses_websocket must be a boolean")
			return
		}
		if enabled {
			filters = append(filters, channelDto.ChannelFilter{Kind: channelDto.FilterResponsesWebSocket})
		}
	}

	preview, err := model.GetRoutingPreview(group, modelName, filters)
	if err != nil {
		common.ApiError(c, err)
		return
	}
	common.ApiSuccess(c, preview)
}
