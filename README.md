# CodeCounter

一个 Windows 原生的代码行数统计小工具：扫描指定目录或文件，自动跳过
`node_modules`、`.idea` 等无关内容，并以「排行榜 + 条形图 + 语言图标」
直观展示各语言的代码量。

> 界面使用 Win32 原生控件（`lxn/walk`），不依赖 WebView，跟随系统主题。

## 功能

- 统计目录或指定文件的代码行数，代码 / 注释 / 空行分别计数
- 默认统计**启动时的工作目录**，可随时切换
- 内置冗余目录排除规则（`node_modules`、`.idea`、`vendor`、`dist` 等），支持自定义
- 排行榜 + 横向条形图，按代码行数降序展示
- 每种语言显示对应图标
- 配置持久化，支持一键重置为默认
- 无控制台窗口，支持高 DPI，支持单实例运行

## 开发状态

项目处于**早期开发阶段**，功能尚未完成。

| 阶段 | 内容 | 状态 |
| --- | --- | --- |
| P0 | 工程骨架与仓库初始化 | 🚧 进行中 |
| P1 | 计数引擎与排除规则 | ⏳ 未开始 |
| P2 | 原生主界面与条形图 | ⏳ 未开始 |
| P3 | 设置对话框与目录选择 | ⏳ 未开始 |
| P4 | 图标、清单与高 DPI | ⏳ 未开始 |
| P5 | CI/CD 与自动发布 | ⏳ 未开始 |
| P6 | 文档与验收 | ⏳ 未开始 |

## 环境要求

- 运行：Windows 10 / 11，无需任何运行时依赖
- 构建：Go 1.25 或更高版本

## 构建

```powershell
git clone https://github.com/linfunss/codecounter.git
cd codecounter
go build -o codecounter.exe ./cmd/codecounter
```

发布版本使用 `-ldflags "-H=windowsgui"` 隐藏控制台窗口，具体见 `Makefile`。

## 文档

- [开发计划](docs/开发计划.md)
- [贡献指南](CONTRIBUTING.md)
- [变更记录](CHANGELOG.md)

## 许可证

[MIT](LICENSE)
