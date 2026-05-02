// Package main is the igo CLI tool.
//
// 当前支持的子命令:
//
//	igo doctor [path]   静态检查 igo 项目，发现常见反模式
//	igo ai ...          输出适合 AI CLI 消费的短上下文
//
// 不实现 igo new / igo gen 之类的脚手架/代码生成器：
// AI 编程工具更适合直接生成代码，CLI 模板维护成本高且容易过时。
package main

import (
	"fmt"
	"os"
)

const usage = `igo - AI-friendly Go web framework

Usage:
  igo <command> [arguments]

Commands:
  doctor [path]    Run static checks for common anti-patterns (default: .)
  ai <subcommand>  Export compact route/context/OpenAPI facts for AI tools
  dev [flags]      Watch + rebuild + restart the app, expose /_ai/dev for AI clients
  release [flags]  Compute next semver from Conventional Commits, write CHANGELOG, tag
  help             Show this help

Examples:
  igo doctor
  igo doctor ./examples/full
  igo ai context ./examples/full
  igo ai routes ./examples/full
  igo ai schemas ./examples/full
  igo ai errors
  igo ai openapi ./examples/full
  igo dev                                # watch cwd, rebuild on save
  igo dev --watcher-port 18999 --dir .   # full flags
  igo release --dry-run                  # preview next version + changelog
  igo release --bump minor               # force bump level
  igo release --push                     # release + push commit/tag
`

func main() {
	if len(os.Args) < 2 {
		fmt.Print(usage)
		os.Exit(1)
	}

	switch os.Args[1] {
	case "doctor":
		path := "."
		if len(os.Args) >= 3 {
			path = os.Args[2]
		}
		os.Exit(runDoctor(path))
	case "ai":
		os.Exit(runAI(os.Args[2:]))
	case "dev":
		os.Exit(runDev(os.Args[2:]))
	case "release":
		os.Exit(runRelease(os.Args[2:]))
	case "help", "-h", "--help":
		fmt.Print(usage)
	default:
		fmt.Fprintf(os.Stderr, "unknown command: %s\n\n", os.Args[1])
		fmt.Print(usage)
		os.Exit(1)
	}
}
