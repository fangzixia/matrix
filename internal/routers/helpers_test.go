package routers

import (
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
)

func TestParamUUIDRejectsInvalidID(t *testing.T) {
	gin.SetMode(gin.TestMode)
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Params = gin.Params{{Key: "id", Value: "not-a-uuid"}}

	if _, ok := paramUUID(c, "id"); ok {
		t.Fatal("expected invalid UUID to be rejected")
	}
	if w.Code != 400 {
		t.Fatalf("status = %d, want 400", w.Code)
	}
}
