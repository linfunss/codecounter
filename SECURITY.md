# 安全策略

## 支持范围

本项目处于早期开发阶段，仅对最新版本修复安全问题。

## 报告漏洞

请不要通过公开 issue 报告安全漏洞。请使用 GitHub 的
[私密漏洞报告](https://github.com/linfunss/codecounter/security/advisories/new)
功能提交，或在仓库 Security 页面选择 "Report a vulnerability"。

报告中请尽量包含：

- 受影响的版本或提交
- 复现步骤
- 影响范围说明
- 可能的修复建议（可选）

收到报告后我们会尽快确认并回复。

## 工具行为说明

CodeCounter 是本地离线工具：

- 不会上传任何统计数据
- 只读取用户指定目录下的文件，不会访问其他位置
- 默认跳过版本控制目录与依赖目录，不会读取其中的内容
- 不会修改被统计的文件
