> **语言**：本文档为简体中文翻译，[点此查看英文原版 README.md](README.md)。

<h1 align="center">Posting</h1>

<p align="center"><strong>一款强大的、常驻终端的 HTTP 客户端。</strong></p>

<p align="center">Posting 是一个 HTTP 客户端，与 Postman 和 Insomnia 颇为相似。作为一个 TUI 应用，它可以通过 SSH 使用，并支持高效的、以键盘为中心的工作流。你的请求以简单的 YAML 文件存储在本地，因此易于阅读，也便于版本管理。</p>

<img width="968" alt="Posting cycling through six dark themes, with request tabs, a populated collection, header autocomplete, and a JSON response" src="docs/assets/themes/themes.gif" />

<p align="center"><a href="https://posting.sh">网站</a> · <a href="#installation">安装</a> · <a href="https://posting.sh/guide/">用户指南</a></p>

一些值得注意的特性包括：

- 请求标签页，让你可以同时处理多个请求
- "跳转模式"导航
- 分层的环境与变量，配有变量界面和自动补全
- 请求头自动补全与集合搜索
- 语法高亮
- 响应搜索、可折叠的 JSON 树，以及 JSONPath 过滤
- HTTP、GraphQL 和 gRPC，包括流式传输
- 服务器推送事件（SSE），配有实时事件日志和 JSON 载荷检查
- 请求计时，以及持久的响应历史
- 可自定义的键位绑定与丰富的配置
- 37 个内置主题，另加用户自定义主题
- 用 $EDITOR/$PAGER 打开
- 把 curl 命令粘贴到 URL 栏即可导入，也可以将请求导出为 curl 命令
- 从 OpenAPI、Postman 和 Bruno 导入
- 命令面板，快速访问各项功能
- `posting remote`，让 Claude Code、Codex 这类编码智能体通过你正打开的 Posting 发送请求，你可以实时旁观

## 跳转模式（Jump mode）

按下快捷键，输入标签字符，即可直接跳转到界面的任何部分。

<img width="968" alt="Filtering the collection, opening a request, jumping to a header field and accepting an autocompletion, then jumping into the response" src="docs/assets/features/jump-mode.gif" />

## 鼠标支持

点击、滚动、选择文本，拖拽以调整面板大小。

<img width="968" alt="Using the mouse to switch request tabs, resize the response panel, scroll through a response, and select text" src="docs/assets/features/mouse.gif" />

## 探索你的响应

搜索响应文本，以可折叠的树结构浏览 JSON，并用 JSONPath 过滤。

<img width="968" alt="Finding text in a JSON response, expanding and collapsing its tree, and filtering the same response with JSONPath" src="docs/assets/features/responses.gif" />

## gRPC 与 GraphQL

发送 GraphQL 查询、对接 gRPC 服务，包括交互式流式传输。

<table>
  <tr>
    <th>gRPC 流式传输</th>
    <th>GraphQL</th>
  </tr>
  <tr>
    <td width="50%"><img width="460" alt="An open bidirectional gRPC stream with sent and received messages and Streaming status" src="docs/assets/features/grpc.png" /></td>
    <td width="50%"><img width="460" alt="A GraphQL query with variables and a successful JSON response" src="docs/assets/features/graphql.png" /></td>
  </tr>
</table>

<a name="installation"></a>

## 安装

Posting 3 目前处于 beta 阶段。它是一个用 Go 编写的单一二进制文件。可以通过 Homebrew 安装：

```bash
brew install darrenburns/homebrew/posting@beta
```

或从[发布页面](https://github.com/darrenburns/posting/releases)下载适用于 macOS、Linux 或 Windows 的压缩包。
或者，在安装了 [Go](https://go.dev/dl/) 1.25.5 或更新版本后，运行：

```bash
go install github.com/darrenburns/posting/v3/cmd/posting@latest
```

现在你就可以通过命令行运行 Posting 了：

```bash
posting
```

从 Posting 2 迁移过来？Posting 3 读取相同的集合、配置和 `.env` 文件。
变更内容参见[从 Posting 2 迁移](https://posting.sh/guide/migrating/)。

## 贡献

欢迎贡献！准则详见 [CONTRIBUTING.md](CONTRIBUTING.md)。要从本仓库的克隆构建 Posting 3，请运行 `go build ./cmd/posting`，并用 `go test ./...` 运行测试。

可以用[示例服务器与集合](examples/grpc/README.md)在本地体验 gRPC，其中包含一个交互式双向流式的完整演示。

Posting 3 基于 [Terma](https://github.com/darrenburns/terma) 构建。Posting 2 则基于 [Textual](https://github.com/textualize/textual) 构建。

<p align="center"><a href="https://posting.sh">网站</a> · <a href="#installation">安装</a> · <a href="https://posting.sh/guide/">用户指南</a></p>
