package routers

import (
	"matrix/internal/app"
	"matrix/internal/modules/identity"
	"matrix/internal/platform/auth"
	platformhttp "matrix/internal/platform/http"

	"github.com/gin-gonic/gin"
)

// registerAuthRoutes 注册认证、用户搜索与个人资料 API。
func registerAuthRoutes(api *gin.RouterGroup, d *app.Deps) {
	api.POST("/auth/login", func(c *gin.Context) { login(c, d) })
	api.POST("/auth/logout", auth.RequireAuth(d.Sessions), func(c *gin.Context) { logout(c, d) })
	api.GET("/auth/me", auth.RequireAuth(d.Sessions), func(c *gin.Context) { me(c, d) })
	api.GET("/users/search", auth.RequireAuth(d.Sessions), func(c *gin.Context) { searchUsers(c, d) })
	api.PUT("/profile", auth.RequireAuth(d.Sessions), func(c *gin.Context) { updateProfile(c, d) })
}

// login 处理用户登录并设置 Session Cookie。
func login(c *gin.Context, d *app.Deps) {
	var body struct {
		Username string `json:"username"`
		Password string `json:"password"`
	}
	if c.BindJSON(&body) != nil {
		platformhttp.JSONError(c, 400, "bad_request", "无效请求")
		return
	}
	u, token, err := d.Auth.Login(c.Request.Context(), body.Username, body.Password, c.ClientIP(), c.Request.UserAgent())
	if err != nil {
		platformhttp.JSONError(c, 401, "unauthorized", "用户名或密码错误")
		return
	}
	c.SetCookie(d.Sessions.CookieName(), token, int(d.Sessions.TTL().Seconds()), "/", "", d.Sessions.Secure(), true)
	c.JSON(200, userResponse(d, u))
}

// userResponse 将用户实体序列化为 API 响应字段。
func userResponse(d *app.Deps, u *identity.User) gin.H {
	rootUser := d.Config.Auth.Bootstrap.AdminUsername
	if rootUser == "" {
		rootUser = "root"
	}
	return gin.H{
		"id":              u.ID,
		"username":        u.Username,
		"email":           u.Email,
		"name":            u.Name,
		"avatar_url":      u.AvatarURL,
		"is_admin":        u.IsAdmin,
		"is_root":         u.Username == rootUser,
		"state":           u.State,
		"last_sign_in_at": u.LastSignInAt,
		"created_at":      u.CreatedAt,
	}
}

// logout 处理用户登出并清除 Session Cookie。
func logout(c *gin.Context, d *app.Deps) {
	token, _ := c.Cookie(d.Sessions.CookieName())
	_ = d.Auth.Logout(c.Request.Context(), token)
	c.SetCookie(d.Sessions.CookieName(), "", -1, "/", "", d.Sessions.Secure(), true)
	c.JSON(200, gin.H{"ok": true})
}

// me 返回当前登录用户信息。
func me(c *gin.Context, d *app.Deps) {
	u, ok := auth.User(c)
	if !ok {
		platformhttp.JSONError(c, 401, "unauthorized", "未登录")
		return
	}
	c.JSON(200, userResponse(d, u))
}

// searchUsers 搜索Users。
func searchUsers(c *gin.Context, d *app.Deps) {
	q := c.Query("q")
	list, err := d.Users.Search(c.Request.Context(), q, 20)
	if err != nil {
		platformhttp.JSONError(c, 500, "internal", err.Error())
		return
	}
	c.JSON(200, gin.H{"users": list})
}

// updateProfile 更新Profile。
func updateProfile(c *gin.Context, d *app.Deps) {
	u, _ := auth.User(c)
	var in identity.UpdateUserInput
	if c.BindJSON(&in) != nil {
		platformhttp.JSONError(c, 400, "bad_request", "无效请求")
		return
	}
	in.IsAdmin = nil
	out, err := d.Users.Update(c.Request.Context(), u.ID, in)
	if err != nil {
		platformhttp.JSONError(c, 400, "bad_request", err.Error())
		return
	}
	c.JSON(200, out)
}
