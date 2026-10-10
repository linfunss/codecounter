# 贡献指南

感谢参与 CodeCounter。本仓库为**公开仓库**，请在提交前完整阅读本文档。

## 一、分支策略

| 分支 | 用途 | 说明 |
| --- | --- | --- |
| `main` | 发布分支 | 只接受来自 `dev` 的合并提交，**禁止直接提交代码** |
| `dev`  | 开发分支 | 所有功能开发、缺陷修复与文档补充都在此进行 |

日常开发流程：

1. 切到 `dev` 并确保与远端同步
2. 在 `dev` 上完成开发与自测
3. 提交前逐项完成「隐私自查清单」
4. 阶段完成后合并到 `main`：

```powershell
git switch dev
git pull --rebase
git switch main
git merge --no-ff -S dev
git push origin main
git switch dev
```

> 代码只允许提交到 `dev`；合并到 `main` 意味着该阶段已通过验收，
> `main` 上的任意提交都应可构建、可发布。

## 二、提交规范

提交信息必须使用**中文**，并遵循 Conventional Commits 前缀：

| 前缀 | 含义 |
| --- | --- |
| `feat` | 新增功能 |
| `fix` | 修复缺陷 |
| `docs` | 文档变更 |
| `test` | 测试相关 |
| `refactor` | 重构（不改变外部行为） |
| `build` | 构建系统或依赖变更 |
| `ci` | CI/CD 配置变更 |
| `chore` | 其他杂项 |

格式如下，标题控制在 50 字以内，正文用中文说明**做了什么、为什么、影响范围**：

```
<类型>: <简短描述>

- 具体做了什么
- 为什么这么做
- 影响范围

影响范围: <受影响的模块或文件>
验证方式: <执行的命令与结果>
Refs: #<issue 编号>
```

示例：

```
feat: 新增目录扫描与按语言聚合统计

- 接入 scc 处理器，支持代码、注释、空行三类计数
- 按代码行数降序生成排行榜数据
- 增加扫描并发控制，默认使用 CPU 核心数

影响范围: internal/scan, internal/counter
验证方式: go test ./internal/... 全部通过；与 scc CLI 抽样结果一致
```

## 三、提交签名

本仓库所有提交与标签**必须使用 GPG 签名**，禁止用 `--no-gpg-sign` 绕过。

建议在仓库本地开启：

```powershell
git config commit.gpgsign true
git config tag.gpgsign true
```

若签名失败，依次执行后重试：

```powershell
gpgconf --launch gpg-agent
gpgconf --launch keyboxd
```

> 在受限沙箱中执行 `git commit` 可能因 DLL 重定位冲突导致 `gpg.exe`
> 无法启动，此时应在沙箱外执行提交。

## 四、隐私自查清单（公开仓库，提交前必做）

- [ ] `git status` 干净，没有意外新增或未跟踪文件
- [ ] `git ls-files` 逐项确认，没有构建产物、本地配置、日志文件
- [ ] 源码、注释、文档中不出现本机绝对路径、用户名、主机名、内网地址
- [ ] 没有提交 token、密钥、`.env`、`config.json`、`*.pem`、`*.key`
- [ ] 不包含任何未获授权的第三方代码、字体或图片资源
- [ ] 新增的第三方依赖已确认许可证兼容（MIT）

## 五、代码风格

- 使用 `gofmt` / `goimports` 格式化
- 注释与面向用户的文案使用中文
- API 路径、字段名、协议字段、业务码保持英文原文
- 提交前执行：

```powershell
gofmt -l .
go vet ./...
go test ./...
golangci-lint run
```

## 六、Windows 资源文件

`cmd/codecounter` 下的 `rsrc_windows_amd64.syso` 与 `rsrc_windows_arm64.syso`
由 `winres/app.manifest` 生成，用于嵌入应用清单（通用控件 6.0、高 DPI 感知、
兼容性声明）。

- 缺少该清单时 Windows 会退回通用控件 v5，界面变成旧式外观，
  并且部分控件 API 调用会直接失败
- 文件名中的 `windows_amd64` / `windows_arm64` 是 Go 的构建约束后缀，
  链接器会按目标架构自动选择
- 修改 `winres/app.manifest` 后必须执行 `go generate ./cmd/codecounter`
  重新生成，并把新的 `.syso` 一并提交

## 七、行为准则

参与本项目即表示同意遵守 [贡献者行为准则](CODE_OF_CONDUCT.md)。
