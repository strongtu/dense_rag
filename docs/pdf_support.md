# PDF 支持方案设计

## 1. 概述

本文档描述了为 dense_rag 项目添加 PDF 文件清洗支持的方案设计，目标是与现有 `.txt` 和 `.docx` 处理流程保持一致。

## 2. 现状分析

### 2.1 当前支持的文件格式

| 格式 | 扩展名 | 状态 |
|------|--------|------|
| 纯文本 | `.txt` | ✅ 已支持 |
| Word文档 | `.docx` | ✅ 已支持 |
| PDF | `.pdf` | ❌ 暂不支持 |

### 2.2 现有架构

```
文件监控 → 文件读取 → 分块 → Embedding → 向量存储 → 检索
            ↑
      cleaning.ReadFile()
```

现有清洗模块位于 `internal/cleaning/`，核心文件：
- `reader.go` - 文件读取与格式转换
- `filter.go` - 文件类型过滤

## 3. 技术方案

### 3.1 PDF 解析库选型

推荐使用以下开源库之一：

| 库 | 优点 | 缺点 |
|----|------|------|
| `github.com/ledongthuc/pdf` | 纯 Go 实现，无依赖；支持文本提取 | 仅支持文本型 PDF |
| `github.com/pdfcpu/pdfcpu` | 功能全面，支持 PDF 操作 | 体积较大 |
| `github.com/jung-kurt/gofpdf` | 生成能力强 | 主要用于生成，非读取 |

**推荐**：使用 `github.com/ledongthuc/pdf`，原因：
1. 纯 Go 实现，部署简单
2. 轻量级，专注于文本提取
3. 社区活跃度较高

### 3.2 核心实现

#### 3.2.1 修改 `filter.go`

在 `IsSupportedFile` 函数中添加 `.pdf` 支持：

```go
func IsSupportedFile(path string) bool {
    base := filepath.Base(path)
    if strings.HasPrefix(base, "~$") {
        return false
    }
    ext := strings.ToLower(filepath.Ext(path))
    return ext == ".txt" || ext == ".docx" || ext == ".pdf"
}
```

#### 3.2.2 修改 `reader.go`

添加 PDF 读取函数：

```go
import (
    "github.com/ledongthuc/pdf"
)

// ReadPdf extracts text content from a PDF file.
// Only supports text-based PDFs (not scanned images).
func ReadPdf(path string) (string, error) {
    f, r, err := pdf.Open(path)
    if err != nil {
        return "", fmt.Errorf("open pdf %s: %w", path, err)
    }
    defer f.Close()

    var buf strings.Builder
    totalPage := r.NumPage()

    for i := 1; i <= totalPage; i++ {
        page := r.Page(i)
        if page.V.IsNull() {
            continue
        }

        text, err := page.GetPlainText(nil)
        if err != nil {
            continue // 跳过无法读取的页面
        }
        buf.WriteString(text)
        buf.WriteString("\n\n")
    }

    return buf.String(), nil
}
```

在 `ReadFile` 函数中添加分发：

```go
func ReadFile(path string) (string, error) {
    ext := strings.ToLower(filepath.Ext(path))
    switch ext {
    case ".txt":
        return ReadTxt(path)
    case ".docx":
        return ReadDocx(path)
    case ".pdf":
        return ReadPdf(path)
    default:
        return "", fmt.Errorf("unsupported file extension: %s", ext)
    }
}
```

#### 3.2.3 添加依赖

```bash
go get github.com/ledongthuc/pdf
```

### 3.3 高级功能设计

> 以下为可选功能，不在 Phase 1 计划内

提取 PDF 元信息供后续使用：

```go
type PdfMetadata struct {
    Title       string
    Author      string
    Subject     string
    Creator     string
    Producer    string
    CreateDate  time.Time
    ModDate     time.Time
    PageCount   int
}

func ExtractPdfMetadata(path string) (*PdfMetadata, error) {
    f, r, err := pdf.Open(path)
    if err != nil {
        return nil, err
    }
    defer f.Close()

    return &PdfMetadata{
        Title:      r.GetTitle(),
        Author:     r.GetAuthor(),
        PageCount:  r.NumPage(),
    }, nil
}
```

#### 3.3.2 目录结构保留（可选）

保留 PDF 的章节结构：

```go
type PdfOutline struct {
    Title    string
    Level    int
    Page     int
}

// ExtractOutline extracts the table of contents from a PDF.
func ExtractOutline(path string) ([]PdfOutline, error) {
    f, r, err := pdf.Open(path)
    if err != nil {
        return nil, err
    }
    defer f.Close()

    // 遍历 PDF Outline
    // ...
}
```

## 4. 测试计划

### 4.1 单元测试

| 测试用例 | 描述 |
|----------|------|
| `TestIsSupportedFile_pdf` | 验证 `.pdf` 文件被正确识别为支持格式 |
| `TestIsSupportedFile_PDF` | 验证大写 `.PDF` 扩展名也被支持 |
| `TestReadPdf_basic` | 测试基本文本 PDF 读取 |
| `TestReadPdf_empty` | 测试空 PDF 处理 |
| `TestReadPdf_password` | 测试加密 PDF 报错处理 |

### 4.2 测试数据

在 `testdata/` 目录下准备测试文件：
- `sample_text.pdf` - 纯文本 PDF
- `sample_encrypted.pdf` - 加密 PDF（可选）

## 5. 配置变更

### 5.1 文件大小限制

PDF 文件通常较大，可能需要调整 `MaxFileSize`：

```go
// 建议保持 20MB 或适当提高至 50MB
var MaxFileSize int64 = 50 * 1024 * 1024
```

### 5.2 配置项（可选）

在 `configs/config.yaml` 中添加 PDF 相关配置：

```yaml
cleaning:
  max_file_size: 52428800  # 50MB
  pdf:
    extract_metadata: true # 是否提取元数据
```

## 6. 实现计划

### Phase 1：基础功能
- [ ] 添加 PDF 库依赖
- [ ] 实现 `ReadPdf` 函数
- [ ] 修改 `filter.go` 支持 PDF
- [ ] 修改 `reader.go` 添加分发逻辑
- [ ] 编写单元测试

### Phase 2：增强功能（可选）
- [ ] 元数据提取
- [ ] 目录结构保留

## 7. 风险与限制

| 风险 | 影响 | 缓解措施 |
|------|------|----------|
| 大型 PDF 内存占用 | 处理超时或内存溢出 | 限制文件大小，分页处理 |
| 特殊编码 PDF | 乱码问题 | 添加编码检测和转换 |
| 扫描版/图片型 PDF | 无法提取文字 | 文档说明仅支持文本型 PDF |

## 8. 附录

### 8.1 相关文件清单

需要修改的文件：
- `internal/cleaning/filter.go`
- `internal/cleaning/reader.go`

需要创建的文件：
- `internal/cleaning/pdf.go`（可选，将 PDF 相关功能独立模块）
- `testdata/sample_text.pdf`

### 8.2 参考资料

- PDF 库：https://github.com/ledongthuc/pdf
- 项目现有实现：`internal/cleaning/reader.go`
