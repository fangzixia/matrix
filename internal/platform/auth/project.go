package auth

import (
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"

	"matrix/internal/modules/iam"
	platformhttp "matrix/internal/platform/http"
)

func RequireProject(enforcer *iam.Enforcer, minRole iam.Role) gin.HandlerFunc {
	return func(c *gin.Context) {
		u, ok := User(c)
		if !ok {
			platformhttp.JSONError(c, 401, "unauthorized", "请先登录")
			return
		}
		pid, err := uuid.Parse(c.Param("id"))
		if err != nil {
			platformhttp.JSONError(c, 400, "bad_request", "无效项目 ID")
			return
		}
		if u.IsAdmin {
			c.Set("project_id", pid)
			c.Next()
			return
		}
		ok2, err := enforcer.CanAccess(c.Request.Context(), u.ID, pid, minRole, false)
		if err != nil || !ok2 {
			platformhttp.JSONError(c, 403, "forbidden", "无权访问该项目")
			return
		}
		c.Set("project_id", pid)
		c.Next()
	}
}

func ProjectID(c *gin.Context) uuid.UUID {
	v, _ := c.Get("project_id")
	id, _ := v.(uuid.UUID)
	return id
}
