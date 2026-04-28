package errors

// ErrorCodeInfo 描述一个框架支持的错误码 + 触发它的 Context helper
// 暴露给 /_ai/errors 端点，让 AI 不必读源码就知道该用哪个 helper 触发哪个状态码
type ErrorCodeInfo struct {
	Code        string `json:"code"`        // 错误码常量值（如 "BAD_REQUEST"）
	StatusCode  int    `json:"statusCode"`  // 对应的 HTTP 状态码
	HelperPlain string `json:"helperPlain"` // 普通版 helper 方法名（不带 err 链）
	HelperWrap  string `json:"helperWrap"`  // 带 err 链的 *Wrap 方法名（无则为空）
	Description string `json:"description"` // 何时使用
}

// ListErrorCodes 返回框架支持的全部错误码 + 对应 Context helper 信息
//
// 这是手工维护的清单，每次新增错误码或 helper 都需要在这里同步一行。
// 测试 TestListErrorCodes 会断言返回的 Code 与 structured.go 中的常量保持一致。
func ListErrorCodes() []ErrorCodeInfo {
	return []ErrorCodeInfo{
		{
			Code:        CodeBadRequest,
			StatusCode:  400,
			HelperPlain: "Context.BadRequest(msg)",
			HelperWrap:  "Context.BadRequestWrap(err, msg)",
			Description: "请求格式错误、缺参数、类型转换失败",
		},
		{
			Code:        CodeUnauthorized,
			StatusCode:  401,
			HelperPlain: "Context.Unauthorized(msg)",
			HelperWrap:  "",
			Description: "未登录、token 无效或已过期",
		},
		{
			Code:        CodeForbidden,
			StatusCode:  403,
			HelperPlain: "Context.Forbidden(msg)",
			HelperWrap:  "",
			Description: "已认证但当前用户无权限访问",
		},
		{
			Code:        CodeNotFound,
			StatusCode:  404,
			HelperPlain: "Context.NotFound(msg)",
			HelperWrap:  "Context.NotFoundWrap(err, msg)",
			Description: "资源不存在；可用 Context.NotFoundIfNotFound / SuccessIfNotNil 哨兵简化",
		},
		{
			Code:        CodeValidation,
			StatusCode:  422,
			HelperPlain: "Context.ValidationError(err)",
			HelperWrap:  "Context.ValidationErrorWrap(err, field, msg)",
			Description: "struct tag 校验失败；BindAndValidate/BindQueryAndValidate 自动触发",
		},
		{
			Code:        CodeInternalError,
			StatusCode:  500,
			HelperPlain: "Context.InternalError(msg)",
			HelperWrap:  "Context.InternalErrorWrap(err, msg, meta)",
			Description: "数据库故障、外部服务失败、未预期错误；务必带 err 链便于排查",
		},
		{
			Code:        CodeInvalidJSON,
			StatusCode:  400,
			HelperPlain: "（内部用）",
			HelperWrap:  "Context.BadRequestWrap(err, ...)",
			Description: "JSON 解析失败；BindJSON 内部触发，由 BadRequestWrap 包装",
		},
	}
}
