package workspace

import (
	"errors"
	"fmt"
)

// ErrSourceFetchFailed 表示 Run 源码（Git clone）在重试耗尽后仍失败。
var ErrSourceFetchFailed = errors.New("source fetch failed")

// SourceFetchError 封装 clone 重试耗尽后的错误，供上层识别为不可 Job 重试。
type SourceFetchError struct {
	Attempts int
	Cause    error
}

func (e *SourceFetchError) Error() string {
	if e == nil {
		return "source fetch failed"
	}
	return fmt.Sprintf("git clone 失败（已重试 %d 次）: %v", e.Attempts, e.Cause)
}

func (e *SourceFetchError) Unwrap() error { return ErrSourceFetchFailed }

// IsSourceFetchError 判断 err 是否为源码获取（Git clone）失败。
func IsSourceFetchError(err error) bool {
	var sfe *SourceFetchError
	return errors.As(err, &sfe) || errors.Is(err, ErrSourceFetchFailed)
}
