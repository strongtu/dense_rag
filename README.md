# Dense RAG

本地文件向量检索服务：监控指定目录下的 txt/docx 文件变化，自动构建向量索引，通过 **HTTP API** 与 **MCP (Model Context Protocol)** 提供语义搜索。单一进程同时提供两种接口。

## 快速开始

### 构建与运行

```bash
make build
# 或: go build -o bin/dense-rag ./cmd/dense-rag

./bin/dense-rag
# 或 make run

# 指定配置文件
./bin/dense-rag -config /path/to/config.yaml
```

### 配置

配置文件路径：`configs/config.yaml` 或 `~/.dense_rag/config.yaml`，YAML 格式。主要字段均可选，未设置时使用默认值。

```yaml
host: "127.0.0.1"           # 监听地址，0.0.0.0 可提供局域网访问
port: 8123                  # 监听端口
topk: 5                     # 查询返回结果数
watch_dirs:                 # 监控目录（支持多个）
  - "~/Documents"
  - "/path/to/another"
model: "text-embedding-bge-m3"
model_endpoint: "http://127.0.0.1:11434"   # OpenAI API 兼容的 embedding 服务
```

**前置依赖**：需有一个 OpenAI API 兼容的 embedding 服务（如 ollama、vllm 部署 bge-m3），并正确配置 `model_endpoint`。详见 `configs/config.example.yaml`。

---

## HTTP API

默认基地址：`http://127.0.0.1:8123`（或配置中的 host:port）。

### GET /health

健康检查，返回服务状态与向量库统计。

```bash
curl http://127.0.0.1:8123/health
```

**响应 (200)**

| 字段 | 类型 | 说明 |
|------|------|------|
| `status` | string | `"ok"` 或 `"degraded"` |
| `vector_count` | int | 向量总数 |
| `indexed_files` | int | 已索引文件数 |
| `store_size_bytes` | int | 向量库占用字节数 |

- `ok` — 服务正常，embedding 可达  
- `degraded` — 服务运行中但 embedding 不可达（已索引内容可查，新文件无法处理）

### POST /query

语义查询，返回最相似的文本片段。

- **Method**: `POST`
- **Content-Type**: `application/json`
- **请求体**: `{"text": "查询文本"}`

```bash
curl -X POST http://127.0.0.1:8123/query \
  -H "Content-Type: application/json" \
  -d '{"text": "如何配置向量检索"}'
```

**成功 (200)**：JSON 数组，按相似度降序。每项字段：

| 字段 | 类型 | 说明 |
|------|------|------|
| `text` | string | 匹配到的文本片段 |
| `file_path` | string | 源文件绝对路径 |
| `score` | float | 余弦相似度（0.0 ~ 1.0） |

结果数量由配置 `topk` 控制；向量库为空时返回 `[]`。

**错误 (400)**：请求体错误或 `text` 为空。  
**错误 (500)**：embedding 调用失败。

---

## MCP 接口

同一服务提供 `POST /mcp`，JSON-RPC 2.0，供 Cursor 等支持 MCP over HTTP 的客户端调用。

### 工具

| 工具 | 说明 |
|------|------|
| **semantic_search** | 语义搜索。参数：`query`（必填）、`top_k`（可选） |
| **get_stats** | 索引统计。无参数 |

### 客户端配置

在客户端中配置 MCP 服务 URL：`http://localhost:8123/mcp`。JSON 配置可参考 `configs/mcp-config.json`。

### 请求示例

**initialize**
```json
{"jsonrpc":"2.0","id":1,"method":"initialize","params":{"protocolVersion":"2024-11-05","capabilities":{},"clientInfo":{"name":"client","version":"1.0"}}}
```

**tools/list**
```json
{"jsonrpc":"2.0","id":2,"method":"tools/list","params":{}}
```

**tools/call（get_stats）**
```json
{"jsonrpc":"2.0","id":3,"method":"tools/call","params":{"name":"get_stats","arguments":{}}}
```

**tools/call（semantic_search）**
```json
{"jsonrpc":"2.0","id":4,"method":"tools/call","params":{"name":"semantic_search","arguments":{"query":"机器学习","top_k":5}}}
```

手动验证：
```bash
curl -s -X POST http://127.0.0.1:8123/mcp -H "Content-Type: application/json" \
  -d '{"jsonrpc":"2.0","id":1,"method":"initialize","params":{}}'
```

---

## 文件监控与持久化

- **监控**：递归监听 `watch_dirs` 下所有子目录；仅处理 `.txt`、`.docx`；忽略大于 20MB 的文件。新增/修改自动清洗、分 chunk、embedding、写入向量库；删除则从向量库移除。事件防抖 200ms；4 个工作线程。
- **持久化**：向量库存于内存并持久化到 `~/.dense_rag/store.gob`。SIGINT/SIGTERM 时自动保存；重启后从磁盘加载并与目录对齐（增量更新）。

---

## 项目结构

```
dense_rag/
├── cmd/dense-rag/main.go    # 服务主入口（HTTP API + MCP）
├── internal/
│   ├── api/                 # HTTP API（gin）
│   ├── mcp/                 # MCP 实现
│   ├── config/              # 配置
│   ├── cleaning/            # 文件读取、分 chunk
│   ├── embedding/           # embedding 客户端
│   ├── store/               # 向量库与持久化
│   └── watcher/             # 文件监听
├── configs/
│   ├── config.example.yaml
│   └── mcp-config.json      # MCP 客户端配置示例
├── test/                    # 集成测试
├── Makefile
└── go.mod
```

---

## 测试

- **单元测试**：`go test ./internal/api/...`（含 MCP 与 HTTP）
- **集成测试**：`go test ./test/... -run TestMCPOverHTTP`（需 embedding 服务可用）

---

## 常见问题

1. **服务无法启动 / 连接失败**  
   检查配置、`model_endpoint` 是否可达、数据目录权限。

2. **搜索结果为空**  
   确认 `watch_dirs` 下已有 .txt/.docx 被索引，且 embedding 服务正常。

3. **MCP 协议错误**  
   请求需为合法 JSON-RPC 2.0，且 `Content-Type: application/json`。

4. **无法加载存储**  
   首次或更换目录后会自动全量同步；确保 `~/.dense_rag/` 可写。
