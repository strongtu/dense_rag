# Dense RAG 服务说明文档

Dense RAG 是一个本地文件向量检索服务，监控指定目录下的 txt/docx 文件变化，自动构建向量索引，并通过 HTTP API 和 MCP (Model Context Protocol) 提供语义搜索能力。

## 快速开始

### 构建与运行

```bash
# 构建 HTTP 服务
make build
# 或者: go build -o bin/dense-rag.exe ./cmd/dense-rag

# 构建 MCP 服务
make build-mcp
# 或者: go build -o bin/dense-rag-mcp.exe ./cmd/dense-rag-mcp

# 构建所有服务
make build-all

# 运行 HTTP 服务（使用默认配置）
go run ./cmd/dense-rag

# 运行 MCP 服务
go run ./cmd/dense-rag-mcp

# 运行（指定配置文件）
go run ./cmd/dense-rag -config /path/to/config.yaml
go run ./cmd/dense-rag-mcp -config /path/to/config.yaml
```

### 配置文件

配置文件路径：`~/.dense_rag/config.yaml`，YAML 格式。所有字段均可选，未设置时使用默认值。

```yaml
host: "127.0.0.1"           # 监听地址，默认 127.0.0.1
port: 8123                   # 监听端口，默认 8123
topk: 5                      # 查询返回结果数，默认 5
watch_dir: "~/Documents"     # 监控目录，默认 ~/Documents
model: "text-embedding-bge-m3"          # embedding 模型名称
model_endpoint: "http://127.0.0.1:11434" # embedding 服务地址（OpenAI API 兼容）
```

### 前置依赖

需要一个 OpenAI API 兼容的 embedding 服务运行在 `model_endpoint` 地址上，例如通过 ollama、vllm、text-embeddings-inference 等部署 bge-m3 模型。

---

## HTTP API

默认基地址：`http://127.0.0.1:8123`

### POST /query

语义查询接口。接收查询文本，返回最相似的文本片段。

#### 请求

- **Method**: `POST`
- **Content-Type**: `application/json`

**请求体**：

| 字段 | 类型 | 必填 | 说明 |
|------|------|------|------|
| `text` | string | 是 | 查询文本 |

**示例**：

```bash
curl -X POST http://127.0.0.1:8123/query \
  -H "Content-Type: application/json" \
  -d '{"text": "如何配置向量检索"}'
```

#### 响应

**成功 (200)**：返回 JSON 数组，每个元素为一条匹配结果，按相似度降序排列。

| 字段 | 类型 | 说明 |
|------|------|------|
| `text` | string | 匹配到的文本片段 |
| `file_path` | string | 源文件的本地绝对路径 |
| `score` | float | 余弦相似度分数（0.0 ~ 1.0） |

```json
[
  {
    "text": "向量检索服务支持配置 topk 参数来控制返回结果数量...",
    "file_path": "/home/user/Documents/guide.txt",
    "score": 0.9231
  },
  {
    "text": "配置文件采用 YAML 格式，存储在 ~/.dense_rag/config.yaml...",
    "file_path": "/home/user/Documents/readme.docx",
    "score": 0.8754
  }
]
```

结果数量由配置项 `topk` 控制，默认返回最多 5 条。如果向量库为空，返回空数组 `[]`。

**错误 (400)**：请求体格式错误或 `text` 为空。

```json
{"error": "text must not be empty"}
```

**错误 (500)**：embedding 服务调用失败。

```json
{"error": "embedding failed: connection refused"}
```

---

### GET /health

健康检查接口。返回服务运行状态和向量库统计信息。

#### 请求

```bash
curl http://127.0.0.1:8123/health
```

#### 响应 (200)

| 字段 | 类型 | 说明 |
|------|------|------|
| `status` | string | 服务状态：`"ok"` 或 `"degraded"` |
| `vector_count` | int | 向量库中的向量总数 |
| `indexed_files` | int | 已索引的文件数 |
| `store_size_bytes` | int | 向量库占用的近似内存字节数 |

```json
{
  "status": "ok",
  "vector_count": 1280,
  "indexed_files": 42,
  "store_size_bytes": 5242880
}
```

`status` 取值说明：
- `"ok"` — 服务正常，embedding 服务可达
- `"degraded"` — 服务运行中但 embedding 服务不可达（已索引内容仍可查询，但新文件无法处理）

---

## 文件监控行为

- 递归监听 `watch_dir` 及所有子目录
- 仅处理 `.txt` 和 `.docx` 文件
- 忽略大于 20MB 的文件
- 文件新增/修改：自动清洗、分 chunk、计算 embedding、写入向量库
- 文件删除：自动从向量库中移除对应向量
- 新增子目录：自动纳入监听范围
- 事件防抖：200ms 窗口内对同一文件的多次变更合并为一次处理
- 并发控制：4 个工作线程的有界工作池，防止 CPU 占满

## 数据持久化

- 向量库保存在内存中，同时持久化到 `~/.dense_rag/store.gob`
- 服务收到 SIGINT/SIGTERM 信号时自动保存
- 重启后从磁盘加载向量库，并扫描目录进行对齐（增量更新变化的文件）

## 项目结构

```
dense_rag/
├── cmd/
│   ├── dense-rag/main.go        # HTTP 服务主入口
│   └── dense-rag-mcp/main.go    # MCP 服务主入口
├── internal/
│   ├── api/                     # HTTP API（gin 框架）
│   ├── mcp/                     # MCP 服务器实现
│   ├── config/                  # 配置管理
│   ├── cleaning/                # 文件读取、docx 清洗、文本分 chunk
│   ├── embedding/               # OpenAI 兼容 embedding 客户端
│   ├── store/                   # 内存向量库、持久化、启动对齐
│   └── watcher/                 # 文件监听、防抖、工作池
├── configs/config.example.yaml  # 配置模板
├── mcp-config.json              # MCP 客户端配置示例
├── MCP_README.md                # MCP 服务详细说明
├── test_mcp.py                  # MCP 服务测试脚本
├── Makefile                     # 构建脚本
└── go.mod
```

---

## MCP (Model Context Protocol) 服务

Dense RAG 还提供了 MCP 服务器，让 AI agents 可以通过标准化的 MCP 协议访问文档向量搜索功能。

### MCP 服务特性

- **标准协议**: 实现 MCP 2024-11-05 版本规范
- **工具集成**: 提供 `semantic_search` 和 `get_stats` 两个工具
- **共享存储**: 与 HTTP 服务共享相同的向量存储
- **JSON-RPC**: 通过 stdin/stdout 进行 JSON-RPC 2.0 通信

### MCP 工具说明

#### 1. semantic_search
- **功能**: 在索引的文档中搜索语义相似的文本块
- **参数**:
  - `query` (必需): 搜索查询文本
  - `top_k` (可选): 返回结果数量，默认使用配置值
- **返回**: 按相似度排序的搜索结果

#### 2. get_stats
- **功能**: 获取索引文档和向量的统计信息
- **参数**: 无
- **返回**: 文件数量、向量数量、存储大小等统计信息

### MCP 使用方法

1. **构建 MCP 服务**:
```bash
go build -o bin/dense-rag-mcp.exe ./cmd/dense-rag-mcp
```

2. **配置 MCP 客户端**:
```json
{
  "mcpServers": {
    "dense-rag": {
      "command": "/path/to/dense-rag-mcp",
      "args": [],
      "env": {}
    }
  }
}
```

3. **测试 MCP 服务**:
```bash
python test_mcp.py
```

### 注意事项

- MCP 服务器不监控文件变化，需要先运行 HTTP 服务建立索引
- MCP 和 HTTP 服务可以同时运行，共享向量存储
- 确保 embedding 模型服务正在运行

详细的 MCP 使用说明请参考 `MCP_README.md` 文件。
