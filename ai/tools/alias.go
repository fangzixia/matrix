// Package tools 提供 LLM 运行期可调用的内置工具实现（read_file、grep、bash 等）。
//
// Tool 类型、Registry、RunTools 等执行框架见 matrix/ai/util。
// 工具流式输出请写 util.StreamWriter(ctx)；生命周期事件由 util.RunTools 负责。
package tools

import (
	"matrix/ai/util"
)

type (
	Tool       = util.Tool
	JSONSchema = util.JSONSchema
	PropSchema = util.PropSchema
	Registry   = util.Registry
	Result     = util.Result
)

const defaultEmitChunkSize = util.DefaultEmitChunkSize

func getString(args map[string]any, key string) (string, bool) {
	return util.GetString(args, key)
}

func getBoolArg(args map[string]any, key string) bool {
	return util.GetBool(args, key)
}
