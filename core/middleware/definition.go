package middleware

// Phase 中间件执行阶段
type Phase string

const (
	PhaseBefore Phase = "BEFORE" // 请求处理前
	PhaseAfter  Phase = "AFTER"  // 请求处理后
)

// Definition 中间件定义
type Definition struct {
	Name         string      `json:"name"`
	Description  string      `json:"description"`
	FilePath     string      `json:"filePath"`
	LineNumber   int         `json:"lineNumber"`
	Phase        Phase       `json:"phase"`         // BEFORE 或 AFTER
	Priority     int         `json:"priority"`     // 0-100, 越高越先执行
	Dependencies []string    `json:"dependencies"`  // 依赖的其他中间件名
	AIHints      []string    `json:"aiHints"`     // AI 调试提示
	Tags         []string    `json:"tags"`         // 分类标签: auth, logging, cors
}

// MiddlewareChain 中间件链定义
type MiddlewareChain struct {
	Items []ChainItem `json:"items"`
}

// ChainItem 链中的中间件项
type ChainItem struct {
	Name       string      `json:"name"`
	Definition *Definition `json:"definition,omitempty"`
	Enabled    bool        `json:"enabled"`
}

// DefaultMiddlewareOrder 默认中间件执行顺序
var DefaultMiddlewareOrder = []string{
	"Recovery",  // 100 - 恢复（最先）
	"CORS",      // 80  - 跨域
	"Auth",      // 70  - 认证
	"RateLimit", // 60  - 限流
	"Logger",    // 50  - 日志
	"RequestID",  // 40  - 请求 ID
}

// DefaultMiddlewareDefinitions 默认中间件定义
var DefaultMiddlewareDefinitions = map[string]Definition{
	"Recovery": {
		Name:        "Recovery",
		Description: "Panic recovery middleware - catches panics and returns 500",
		Phase:       PhaseBefore,
		Priority:    100,
		Dependencies: []string{},
		AIHints:     []string{"Never skip Recovery middleware"},
		Tags:        []string{"core", "safety"},
	},
	"CORS": {
		Name:        "CORS",
		Description: "Cross-Origin Resource Sharing - handles preflight requests",
		Phase:       PhaseBefore,
		Priority:    80,
		Dependencies: []string{"Recovery"},
		AIHints:     []string{"Must be before Auth for preflight to work"},
		Tags:        []string{"cors", "security"},
	},
	"Auth": {
		Name:        "Auth",
		Description: "JWT authentication - validates Bearer tokens",
		Phase:       PhaseBefore,
		Priority:    70,
		Dependencies: []string{"Recovery", "CORS"},
		AIHints:     []string{"Add X-User-ID header after successful auth"},
		Tags:        []string{"auth", "security"},
	},
	"RateLimit": {
		Name:        "RateLimit",
		Description: "Rate limiting - prevents abuse",
		Phase:       PhaseBefore,
		Priority:    60,
		Dependencies: []string{"Recovery"},
		AIHints:     []string{"Returns 429 when limit exceeded"},
		Tags:        []string{"rate-limit", "security"},
	},
	"Logger": {
		Name:        "Logger",
		Description: "Request logging - logs method, path, status, duration",
		Phase:       PhaseAfter,
		Priority:    50,
		Dependencies: []string{},
		AIHints:     []string{"Logs after handler completes"},
		Tags:        []string{"logging", "observability"},
	},
	"RequestID": {
		Name:        "RequestID",
		Description: "Request ID generation - adds X-Request-ID header",
		Phase:       PhaseBefore,
		Priority:    40,
		Dependencies: []string{},
		AIHints:     []string{"Generates UUID if not provided"},
		Tags:        []string{"logging", "observability"},
	},
}

// GetDefinition 获取中间件定义
func GetDefinition(name string) *Definition {
	if def, ok := DefaultMiddlewareDefinitions[name]; ok {
		return &def
	}
	return nil
}

// ListDefinitions 列出所有中间件定义
func ListDefinitions() []Definition {
	defs := make([]Definition, 0, len(DefaultMiddlewareDefinitions))
	for _, def := range DefaultMiddlewareDefinitions {
		defs = append(defs, def)
	}
	return defs
}

// ValidateOrder 验证中间件顺序是否正确
func ValidateOrder(middlewares []string) []string {
	var warnings []string

	for i := 0; i < len(middlewares)-1; i++ {
		curr := middlewares[i]
		currDef := GetDefinition(curr)
		if currDef == nil {
			continue
		}

		for j := i + 1; j < len(middlewares); j++ {
			next := middlewares[j]
			nextDef := GetDefinition(next)
			if nextDef == nil {
				continue
			}

			// 检查依赖关系
			for _, dep := range currDef.Dependencies {
				if next == dep && nextDef.Priority < currDef.Priority {
					warnings = append(warnings,
						curr+" requires "+dep+" to run before it, but "+next+" has lower priority")
				}
			}
		}
	}

	return warnings
}
