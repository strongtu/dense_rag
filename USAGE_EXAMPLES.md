# Dense RAG 使用示例

本文档提供了 Dense RAG HTTP 服务和 MCP 服务的具体使用示例。

## HTTP 服务示例

### 1. 启动服务

```bash
# 使用默认配置启动
./bin/dense-rag.exe

# 使用自定义配置启动
./bin/dense-rag.exe -config /path/to/config.yaml
```

### 2. 健康检查

```bash
curl http://127.0.0.1:8123/health
```

响应示例：
```json
{
  "status": "ok",
  "vector_count": 1280,
  "indexed_files": 42,
  "store_size_bytes": 5242880
}
```

### 3. 语义搜索

```bash
curl -X POST http://127.0.0.1:8123/query \
  -H "Content-Type: application/json" \
  -d '{"text": "如何配置向量检索服务"}'
```

响应示例：
```json
[
  {
    "text": "向量检索服务支持配置 topk 参数来控制返回结果数量，默认值为5。可以在配置文件中设置：topk: 10",
    "file_path": "C:\\Users\\user\\Documents\\config_guide.txt",
    "score": 0.9231
  },
  {
    "text": "配置文件采用 YAML 格式，存储在 ~/.dense_rag/config.yaml 路径下。支持的配置项包括：host、port、topk、watch_dirs等。",
    "file_path": "C:\\Users\\user\\Documents\\readme.txt",
    "score": 0.8754
  }
]
```

## MCP 服务示例

### 1. 启动 MCP 服务

```bash
# 直接启动
./bin/dense-rag-mcp.exe

# 使用自定义配置
./bin/dense-rag-mcp.exe -config /path/to/config.yaml
```

### 2. MCP 客户端配置

在你的 MCP 客户端配置文件中添加：

```json
{
  "mcpServers": {
    "dense-rag": {
      "command": "C:\\path\\to\\dense-rag-mcp.exe",
      "args": [],
      "env": {}
    }
  }
}
```

### 3. MCP 协议交互示例

#### 初始化请求
```json
{
  "jsonrpc": "2.0",
  "id": 1,
  "method": "initialize",
  "params": {
    "protocolVersion": "2024-11-05",
    "capabilities": {},
    "clientInfo": {
      "name": "my-ai-client",
      "version": "1.0.0"
    }
  }
}
```

#### 列出可用工具
```json
{
  "jsonrpc": "2.0",
  "id": 2,
  "method": "tools/list",
  "params": {}
}
```

响应示例：
```json
{
  "jsonrpc": "2.0",
  "id": 2,
  "result": {
    "tools": [
      {
        "name": "semantic_search",
        "description": "Search for semantically similar text chunks in the indexed documents",
        "inputSchema": {
          "type": "object",
          "properties": {
            "query": {
              "type": "string",
              "description": "The search query text"
            },
            "top_k": {
              "type": "integer",
              "description": "Number of top results to return (optional, default from config)",
              "minimum": 1,
              "maximum": 100
            }
          },
          "required": ["query"]
        }
      },
      {
        "name": "get_stats",
        "description": "Get statistics about the indexed documents and vectors",
        "inputSchema": {
          "type": "object",
          "properties": {},
          "required": []
        }
      }
    ]
  }
}
```

#### 获取统计信息
```json
{
  "jsonrpc": "2.0",
  "id": 3,
  "method": "tools/call",
  "params": {
    "name": "get_stats",
    "arguments": {}
  }
}
```

响应示例：
```json
{
  "jsonrpc": "2.0",
  "id": 3,
  "result": {
    "content": [
      {
        "type": "text",
        "text": "Dense RAG Statistics:\n- Indexed Files: 42\n- Vector Count: 1280\n- Store Size: 5242880 bytes\n"
      }
    ]
  }
}
```

#### 语义搜索
```json
{
  "jsonrpc": "2.0",
  "id": 4,
  "method": "tools/call",
  "params": {
    "name": "semantic_search",
    "arguments": {
      "query": "如何配置向量检索服务",
      "top_k": 3
    }
  }
}
```

响应示例：
```json
{
  "jsonrpc": "2.0",
  "id": 4,
  "result": {
    "content": [
      {
        "type": "text",
        "text": "Found 3 results for query '如何配置向量检索服务':\n\nResult 1 (Score: 0.9231):\nFile: C:\\Users\\user\\Documents\\config_guide.txt\nText: 向量检索服务支持配置 topk 参数来控制返回结果数量，默认值为5。可以在配置文件中设置：topk: 10\n\nResult 2 (Score: 0.8754):\nFile: C:\\Users\\user\\Documents\\readme.txt\nText: 配置文件采用 YAML 格式，存储在 ~/.dense_rag/config.yaml 路径下。支持的配置项包括：host、port、topk、watch_dirs等。\n\nResult 3 (Score: 0.8123):\nFile: C:\\Users\\user\\Documents\\setup.txt\nText: 启动服务前需要确保 embedding 模型服务正在运行，可以使用 ollama、vllm 等工具部署 bge-m3 模型。\n\n"
      }
    ]
  }
}
```

## 测试脚本

### Python 测试脚本
使用提供的 `test_mcp.py` 脚本测试 MCP 服务：

```bash
python test_mcp.py
```

### PowerShell 演示脚本
使用提供的 `demo_mcp.ps1` 脚本启动并演示 MCP 服务：

```powershell
.\demo_mcp.ps1
```

### 批处理测试脚本
使用提供的 `test_mcp.bat` 脚本进行简单测试：

```cmd
test_mcp.bat
```

## 常见问题

### 1. MCP 服务无法启动
- 检查配置文件是否正确
- 确保 embedding 模型服务正在运行
- 检查是否有足够的权限访问数据目录

### 2. 搜索结果为空
- 确保已经有文档被索引（先运行 HTTP 服务监控文件变化）
- 检查 watch_dirs 配置是否正确
- 确认目标目录中有 .txt 或 .docx 文件

### 3. embedding 服务连接失败
- 检查 model_endpoint 配置是否正确
- 确认 embedding 服务正在运行并可访问
- 检查网络连接和防火墙设置

### 4. MCP 协议错误
- 确保发送的是有效的 JSON-RPC 2.0 格式请求
- 检查请求中的 method 和 params 是否正确
- 查看服务器日志获取详细错误信息