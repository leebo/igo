package metadata

import (
	"regexp"
	"strconv"
	"strings"
)

const (
	annotationPrefix = "// igo:"
	linePrefix       = "// "
)

// 注解正则表达式
var (
	summaryRe      = regexp.MustCompile(`^// igo:summary:\s*(.+)$`)
	descriptionRe  = regexp.MustCompile(`^// igo:description:\s*(.+)$`)
	paramRe        = regexp.MustCompile(`^// igo:param:\s*(\w+):(\w+):(\w+):\s*(.+)$`)
	responseRe     = regexp.MustCompile(`^// igo:response:\s*(\d+):\s*(.+)$`)
	tagRe          = regexp.MustCompile(`^// igo:tag:\s*(.+)$`)
	aiHintRe       = regexp.MustCompile(`^// igo:ai-hint:\s*(.+)$`)
	requestBodyRe  = regexp.MustCompile(`^// igo:request-body:\s*(.+)$`)
	deprecatedRe   = regexp.MustCompile(`^// igo:deprecated(?:\s+(.+))?$`)
)

// ParseHandlerDoc 解析处理函数的文档注释
// 支持的注解格式:
//
//	// igo:summary: Get user by ID
//	// igo:description: Returns user details including profile
//	// igo:param:id:path:int:User ID
//	// igo:param:email:query:string:User email
//	// igo:response:200:User object
//	// igo:response:404:User not found
//	// igo:tag:users
//	// igo:ai-hint: Check if user exists before returning
//	// igo:deprecated: Use /v2/users instead
func ParseHandlerDoc(doc string) *RouteMeta {
	lines := strings.Split(doc, "\n")
	meta := &RouteMeta{
		Parameters: make([]ParamMeta, 0),
		Responses:  make([]ResponseMeta, 0),
		Tags:       make([]string, 0),
		AIHints:    make([]string, 0),
	}

	for _, line := range lines {
		line = strings.TrimSpace(line)

		if matches := summaryRe.FindStringSubmatch(line); len(matches) > 1 {
			meta.Summary = matches[1]
			continue
		}

		if matches := descriptionRe.FindStringSubmatch(line); len(matches) > 1 {
			meta.Description = matches[1]
			continue
		}

		// igo:param:name:in:type:description
		if matches := paramRe.FindStringSubmatch(line); len(matches) > 4 {
			required := true
			if matches[2] == "query" {
				required = false
			}
			meta.Parameters = append(meta.Parameters, ParamMeta{
				Name:        matches[1],
				In:          matches[2],
				Type:        matches[3],
				Required:    required,
				Description: matches[4],
			})
			continue
		}

		// igo:response:statusCode:description
		if matches := responseRe.FindStringSubmatch(line); len(matches) > 2 {
			statusCode, _ := strconv.Atoi(matches[1])
			meta.Responses = append(meta.Responses, ResponseMeta{
				StatusCode:  statusCode,
				Description: matches[2],
			})
			continue
		}

		// igo:tag:tagName
		if matches := tagRe.FindStringSubmatch(line); len(matches) > 1 {
			meta.Tags = append(meta.Tags, matches[1])
			continue
		}

		// igo:ai-hint:hint
		if matches := aiHintRe.FindStringSubmatch(line); len(matches) > 1 {
			meta.AIHints = append(meta.AIHints, matches[1])
			continue
		}

		// igo:request-body:description
		if matches := requestBodyRe.FindStringSubmatch(line); len(matches) > 1 {
			meta.RequestBody = &BodyMeta{
				ContentType: "application/json",
				Description: matches[1],
			}
			continue
		}

		// igo:deprecated
		if matches := deprecatedRe.FindStringSubmatch(line); len(matches) > 0 {
			meta.Deprecated = true
			continue
		}
	}

	return meta
}

// ExtractDocFromSource 从源代码中提取处理函数的文档
// 需要传入源代码文件路径和行号
func ExtractDocFromSource(lines []string, startLine, endLine int) string {
	if startLine < 1 || startLine > endLine {
		return ""
	}

	var doc strings.Builder
	for i := startLine - 1; i < endLine && i < len(lines); i++ {
		line := strings.TrimSpace(lines[i])
		if strings.HasPrefix(line, "//") && !strings.HasPrefix(line, "// igo:") {
			// 普通注释行
			doc.WriteString(line[2:])
			doc.WriteString("\n")
		} else if strings.HasPrefix(line, "// igo:") {
			// igo 注解行
			doc.WriteString(line)
			doc.WriteString("\n")
		} else if line == "" {
			continue
		} else {
			break
		}
	}
	return doc.String()
}

// BuildRouteMetaFromAnnotations 从注解构建 RouteMeta
// filePath 参数格式: handlers/user.go:42
func BuildRouteMetaFromAnnotations(sourceLines []string, method, path string, filePath string, startLine int) *RouteMeta {
	meta := &RouteMeta{
		Method:     method,
		Path:       path,
		HandlerName: "anonymous",
		FilePath:   filePath,
		LineNumber: startLine,
		Parameters: make([]ParamMeta, 0),
		Responses:  make([]ResponseMeta, 0),
		Tags:       make([]string, 0),
		AIHints:    make([]string, 0),
	}

	lines := sourceLines

	// 向前查找注解
	for i := startLine - 2; i >= 0; i-- {
		line := strings.TrimSpace(lines[i])
		if line == "" {
			continue
		}
		if strings.HasPrefix(line, "// igo:") {
			// 收集注解
			doc := line + "\n"
			for j := i - 1; j >= 0; j-- {
				prevLine := strings.TrimSpace(lines[j])
				if strings.HasPrefix(prevLine, "// igo:") {
					doc = prevLine + "\n" + doc
					i = j
				} else if strings.HasPrefix(prevLine, "//") {
					doc = prevLine + "\n" + doc
					i = j
				} else {
					break
				}
			}
			parsed := ParseHandlerDoc(doc)
			meta.Summary = coalesce(parsed.Summary, meta.Summary)
			meta.Description = coalesce(parsed.Description, meta.Description)
			meta.Parameters = parsed.Parameters
			meta.Responses = parsed.Responses
			meta.Tags = parsed.Tags
			meta.AIHints = parsed.AIHints
			meta.Deprecated = parsed.Deprecated
			meta.RequestBody = parsed.RequestBody
			break
		} else if strings.HasPrefix(line, "//") {
			continue
		} else {
			break
		}
	}

	return meta
}

func coalesce(s1, s2 string) string {
	if s1 != "" {
		return s1
	}
	return s2
}
