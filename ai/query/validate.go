package query

import (
	"fmt"
	"strings"
)

// Validate 校验 Config 必填字段。
func (c Config) Validate() error {
	if c.LLM == nil {
		return fmt.Errorf("query.Config: LLM is required")
	}
	if strings.TrimSpace(c.Model) == "" {
		return fmt.Errorf("query.Config: Model is required")
	}
	if strings.TrimSpace(c.ThreadID) == "" {
		return fmt.Errorf("query.Config: ThreadID is required")
	}
	return nil
}
