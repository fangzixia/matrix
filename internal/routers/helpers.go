package routers

import (
	platformhttp "matrix/internal/platform/http"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

func paramUUID(c *gin.Context, name string) (uuid.UUID, bool) {
	id, err := uuid.Parse(c.Param(name))
	if err != nil {
		platformhttp.JSONError(c, 400, "bad_request", "无效的 ID")
		return uuid.Nil, false
	}
	return id, true
}

func bindJSON(c *gin.Context, dst any) bool {
	if err := c.BindJSON(dst); err != nil {
		platformhttp.JSONError(c, 400, "bad_request", "无效请求")
		return false
	}
	return true
}
