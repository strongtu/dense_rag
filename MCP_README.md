# Dense RAG MCP Server

Dense RAG MCP Server 提供了 Model Context Protocol (MCP) 接口，让 AI agents 可以通过标准化协议访问文档向量搜索功能。

## 功能特性

MCP服务器提供以下工具：

### 1. semantic_search
- **描述**: 在索引的文档中搜索语义相似的文本块
- **参数**:
  - `query` (必需): 搜索查询文本
  - `top_k` (可选): 返回的结果数量，默认使用配置文件中的值
- **返回**: 按相似度排序的搜索结果列表

### 2. get_stats
- **描述**: 获取索引文档和向量的统计信息
- **参数**: 无
- **返回**: 包含文件数量、向量数量和存储大小的统计信息

## 安装和构建

1. 构建MCP服务器：
```bash
make build-mcp
```

2. 或者构建所有组件：
```bash
make build-all
```

## 配置

MCP服务器使用与HTTP服务相同的配置文件 (`~/.dense_rag/config.yaml`)。确保以下配置项正确设置：

```yaml
model: "text-embedding-bge-m3"
model_endpoint: "http://127.0.0.1:11434"
topk: 5
watch_dirs:
  - "~/Documents"
  - "~/Projects"
```

## 使用方法

### 1. 直接运行
```bash
./bin/dense-rag-mcp
```

### 2. 使用Makefile
```bash
make run-mcp
```

### 3. 与AI客户端集成

将以下配置添加到你的MCP客户端配置中：

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

## 工作流程

1. **启动服务**: MCP服务器启动时会加载已索引的文档向量数据
2. **接收请求**: 通过stdin接收JSON-RPC 2.0格式的MCP请求
3. **处理工具调用**: 
   - `semantic_search`: 将查询文本向量化，然后在向量存储中搜索相似文档
   - `get_stats`: 返回当前索引状态的统计信息
4. **返回结果**: 通过stdout返回JSON格式的响应

## 示例

### 语义搜索示例
```json
{
  "jsonrpc": "2.0",
  "id": 1,
  "method": "tools/call",
  "params": {
    "name": "semantic_search",
    "arguments": {
      "query": "machine learning algorithms",
      "top_k": 3
    }
  }
}
```

### 获取统计信息示例
```json
{
  "jsonrpc": "2.0",
  "id": 2,
  "method": "tools/call",
  "params": {
    "name": "get_stats",
    "arguments": {}
  }
}
```

## 注意事项

1. **数据同步**: MCP服务器使用与HTTP服务相同的向量存储，但不会监控文件变化。如需更新索引，请先运行HTTP服务进行文件监控和索引更新。

2. **并发使用**: MCP服务器和HTTP服务可以同时运行，它们共享相同的向量存储文件。

3. **配置文件**: 确保配置文件中的embedding模型服务正在运行并可访问。

## 故障排除

1. **无法加载存储文件**: 检查 `~/.dense_rag/store.gob` 文件是否存在，如不存在请先运行HTTP服务建立索引。

2. **embedding服务连接失败**: 检查配置文件中的 `model_endpoint` 是否正确，确保embedding服务正在运行。

3. **MCP协议错误**: 确保客户端发送的是有效的JSON-RPC 2.0格式请求。