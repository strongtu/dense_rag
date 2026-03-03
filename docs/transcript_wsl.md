# WSL2 Poll Watcher 实施对话日志

## 对话概要

- **目标**: 根据 `dense_wsl.txt` 需求，为 dense-rag 在 WSL2 环境下新增 poll-based 文件监控能力
- **变更规模**: 1138 行新增，160 行删除
- **最终结果**: 68 个单元测试 + 17 个集成测试全部通过

---

## 第一轮：需求评估

**用户**: dense_wsl.txt 中详细描述了在 wsl 环境中的功能优化需求，请评估技术可行性和有待完善的细节

**AI 评估结果**:

### 需求概要

在 WSL2 环境下，通过轮询机制监控 Windows 挂载目录（`/mnt/xxx`）的文件变化，弥补 fsnotify/inotify 在 9P 协议下失效的问题。

### 现有架构分析

当前实现基于 `fsnotify` (`internal/watcher/watcher.go`)，事件流为：

```
fsnotify.Event → handleEvent() → Filter(.txt/.docx) → Debouncer(200ms) → WorkerPool(4) → processFn
```

配置中仅支持单个 `watch_dir`，无 WSL 环境检测、无轮询机制。

### 技术可行性判断

| 需求项 | 可行性 | 说明 |
|--------|--------|------|
| 轮询替代 inotify | **可行** | Go 标准库 `os.Stat` / `filepath.WalkDir` 即可实现，无需第三方依赖 |
| 目录 ModTime 快速判定 | **部分可行，有风险** | 见下方详细分析 |
| 文件 ModTime + Size 双重校验 | **可行** | `os.FileInfo` 提供 `ModTime()` 和 `Size()` |
| 纯 Go 标准库实现 | **可行** | 轮询方案本身不依赖 OS 特有 API |
| WSL 运行时检测 | **可行** | 读取 `/proc/version` 检查是否包含 `microsoft` 或 `WSL` 字符串 |
| 递归子目录监控 | **可行** | `filepath.WalkDir` 标准做法 |
| 1000-2000 文件、1s 轮询 | **可行** | 9P 文件系统下 stat 调用有额外开销，但此量级可接受 |

### 需要完善的细节与风险点

#### 1. 目录 ModTime 快速判定的可靠性问题（重要）

文档第三节第2点提到"先校验根目录 ModTime，无变化则跳过全量扫描"。这个优化**在 9P/CIFS 文件系统上不可靠**：

- **Linux 原生文件系统**：子目录中新增/删除文件**不会**更新父目录的 ModTime，只更新直接包含该文件的目录的 ModTime。
- **9P 协议（WSL2 挂载）**：ModTime 行为可能更不一致，Windows NTFS 的目录时间戳语义与 Linux 不同。
- **结论**：仅检查根目录 ModTime 无法可靠跳过子目录扫描。建议改为**检查每一级目录的 ModTime**，或直接放弃此优化，在 1000-2000 文件量级下全量 `WalkDir` 的开销在 1s 轮询间隔内完全可以承受。

#### 2. 配置层面缺少多目录支持

当前 `config.yaml` 只有一个 `watch_dir` 字段。需求中提到"支持额外的监控目录"，需要明确：

- `watch_dir` 是否改为列表（`watch_dirs: [...]`）？
- WSL 轮询目录与原生 inotify 目录是否共存？
- 建议：配置支持多目录，运行时根据路径前缀 `/mnt/` 自动判定使用轮询还是 fsnotify。

#### 3. WSL 环境检测的粒度

文档说"运行时判断处于 WSL 环境才运行优化策略"，但更准确的判断应该是**按目录路径**而非全局环境：

- 即使在 WSL 中，`~/Documents`（WSL 原生 ext4）的 inotify 是正常工作的。
- 只有 `/mnt/xxx` 路径才需要轮询。
- 建议：WSL 检测 + 路径前缀 `/mnt/` 双重判断，决定该目录使用哪种 watcher。

#### 4. 文件修改检测的边界情况

ModTime + Size 双重校验存在一个盲区：

- **文件内容变化但 Size 不变、ModTime 被保留**：某些编辑器或同步工具可能出现此情况（概率极低但存在）。
- **9P 时间戳精度**：Windows NTFS 时间戳精度为 100ns，但通过 9P 映射到 Linux 后精度可能降低。
- 建议：如果未来发现漏检，可考虑对小文件补充 hash 校验（但当前阶段 ModTime + Size 已足够）。

#### 5. 事件与现有 Pipeline 的对接方式

- 轮询本身已有 1s 间隔，是否还需要 Debouncer？建议**轮询模式跳过 Debouncer**，因为轮询本身就是批量比对。
- 事件格式应统一为现有的 `watcher.EventOp`（`OpCreateModify` / `OpDelete`），确保下游 `processFn` 无需修改。

#### 6. "状态原子更新"的具体实现

- 轮询器的状态缓存是单 goroutine 写入的，天然无竞争。
- 建议轮询完成 diff 后，**立即更新快照**（而非等处理完成再更新），避免同一文件被重复提交。

#### 7. 安全退出机制

- 现有 `watcher.go` 通过 `context.Context` 控制生命周期。
- 轮询器应复用相同的 `ctx`，在 `ctx.Done()` 时退出 `time.Ticker` 循环。

#### 8. 缺少的设计细节

- **首次启动时的全量扫描**：Poll Watcher 启动时快照为空，第一轮会把所有文件视为"新增"。建议复用 reconcile。
- **符号链接处理**：`WalkDir` 默认不跟随符号链接，需确认是否需要支持。
- **隐藏文件/临时文件**：Windows 下 `~$*.docx`（Word 临时文件）、`Thumbs.db` 等应明确排除。
- **大量文件同时变化**：WorkerPool(4) 的背压策略需要确认。

### 推荐架构

```
                     ┌─────────────────────┐
                     │   config.watch_dirs  │
                     │  (多目录列表)         │
                     └──────┬──────────────┘
                            │
               ┌────────────┴────────────┐
               │ 按路径判断 watcher 类型   │
               │ /mnt/* → PollWatcher    │
               │ 其他   → FsnotifyWatcher│
               └────┬───────────┬────────┘
                    │           │
            ┌───────▼──┐  ┌────▼───────┐
            │ PollWatcher│  │FsnotifyWatcher│
            │ (1s轮询)   │  │(现有实现)      │
            └───────┬──┘  └────┬───────┘
                    │          │
                    └────┬─────┘
                         │ 统一 EventOp
                    ┌────▼─────┐
                    │WorkerPool│
                    └────┬─────┘
                         │
                    ┌────▼─────┐
                    │processFn │
                    └──────────┘
```

### 总结

需求**技术上完全可行**，Go 标准库足以实现全部功能。核心风险在于**目录 ModTime 快速判定在 9P 上不可靠**，建议实测验证或直接采用全量扫描。最需要补充的设计细节是：多目录配置方案、轮询/inotify 自动选择策略、首次启动状态对齐、以及临时文件排除规则。

---

## 第二轮：用户补充需求后再评估

**用户**: 我在源文件补充调整了一些细节，你再看一下

用户新增了"零、额外补充需求点"：

```
零、额外补充需求点
1. 原有config.yaml逻辑中，watch_dir要改为支持路径数组
2. 需要对多目录做保护判断，避免用户配置有误，出现A是B的子目录的配置情况
3. 只有 'WSL环境'+'/mnt/xxx'形式的路径，才开启优化
4. 减少对原有监听机制pineline代码架构的侵入，如：
    a.保留原有Notify Watcher，新增轮询Watcher
    b.根据目录性质不同采用不同Watcher，避免交叉工作
    c.Watcher之后的处理工作，如清洗、向量计算应当是统一的流程
```

同时删掉了第三节第2点"目录 ModTime 快速判定"。

**AI 评估**:

**1. `watch_dir` 改为路径数组** — 可行，无风险

**2. 子目录包含关系保护** — 可行，细节需确认：
- 路径需先做 `filepath.Abs()` + `filepath.Clean()` 标准化
- 符号链接场景需用 `filepath.EvalSymlinks()` 解析后再比较
- 发现冲突时的行为需明确：建议启动时直接报错

**3. 仅 "WSL 环境" + "/mnt/" 路径 才开启轮询** — 可行，与上次建议一致

**4. 低侵入设计（a/b/c 三点）** — 可行，与推荐架构一致

**仍待明确的细节**:

1. **首次启动对齐**：建议复用 reconcile 处理初始同步
2. **临时文件排除**：`~$*.docx` 应排除
3. **轮询模式是否经过 Debouncer**：建议跳过
4. **配置格式向后兼容**：建议支持旧 `watch_dir` fallback

---

## 第三轮：用户再次补充

**用户**: 我在原文件补充调整了一些细节，你再看一下

本轮变更：

| 变更点 | 旧版 | 新版 |
|--------|------|------|
| 第零节第1点 | 仅提 watch_dir 改数组 | 追加"同时支持旧配置格式单目录的解析" |
| 第零节第4点 | a/b/c 三条 | 新增 **d**：首次启动统一 reconcile，之后才启动各 watcher |
| 新增第零节第5点 | 无 | 轮询 Watcher 也要过 Debouncer |
| 第四节第1点 | 轮询间隔 1000ms (1秒) | 改为 **10000ms (10秒)** |
| 第四节第2点 | 目录 ModTime 快速判定 | **已删除** |

**AI 评估**:

**1. 旧配置兼容** — Go 的 `yaml.v3` 支持自定义 `UnmarshalYAML`，可同时处理 string 和 `[]string`。

**2. 首次启动统一 reconcile 再启动 watcher** — 可行。流程变为：
```
启动 → reconcile(所有目录) → 初始化各 watcher 快照 → 启动 watcher
```
需注意 Poll Watcher 需拿到当前文件状态作为初始快照。

**3. 轮询 Watcher 也过 Debouncer** — 可行，过 Debouncer 不会有副作用，保持架构统一合理。

**4. 轮询间隔调整为 10s** — 可行，文档类文件 10s 延迟完全可接受。

**5. 删除目录 ModTime 快速判定** — 合理。但第四节第2点仍保留了此条描述，存在矛盾，建议删除。

**仍有待明确的两个细节**:

1. `~$*.docx` 临时文件排除
2. 子目录包含冲突的处理策略（启动时报错 vs 自动取父目录）

---

## 第四轮：创建 OpenSpec 草案

**用户**: `/openspec:proposal` 根据 dense_wsl.txt 中的需求创建草案

**AI 创建了完整的 OpenSpec 变更提案**:

### 目录结构

```
openspec/changes/add-wsl-poll-watcher/
├── proposal.md      # 变更概要
├── design.md        # 6个关键架构决策
├── tasks.md         # 8个阶段30+实施项
└── specs/
    ├── multi-dir-config/spec.md   # 多目录配置规格
    ├── wsl-detection/spec.md      # WSL检测规格
    └── poll-watcher/spec.md       # 轮询监控规格
```

### design.md 关键决策

1. **Watcher 接口抽取**: 提取 `DirWatcher` interface (`Start` + `Stop`)
2. **按目录独立分配 watcher**: WSL + `/mnt/` → Poll，其余 → fsnotify
3. **全量 WalkDir**: 不依赖目录 ModTime 快速判定
4. **Poll 事件也过 Debouncer**: 保持架构统一
5. **自定义 YAML 反序列化**: 兼容旧 `watch_dir` 单字符串格式
6. **启动时统一 reconcile**: PollWatcher 从 reconcile 结果初始化快照

### 验证

```
$ openspec validate add-wsl-poll-watcher --strict --no-interactive
Change 'add-wsl-poll-watcher' is valid
```

---

## 第五轮：实施

**用户**: `/openspec:apply`

### 实施过程

#### Task 1: Multi-directory config support

修改 `internal/config/config.go`:
- `WatchDir string` → `WatchDirs []string`
- 新增 `rawConfig` 结构体支持 YAML 双格式解析
- `watch_dirs` 优先于 `watch_dir`
- 新增 `validateNoOverlap()` 目录重叠检测
- 新增 `isAncestorOrEqual()` 确保 `/mnt/c` 不误匹配 `/mnt/cdrom`
- 新增 `expandTildeDirs()` 批量路径展开

更新测试 `config_test.go`:
- `TestLoad_LegacyWatchDir` — 旧格式兼容
- `TestLoad_WatchDirsArray` — 新数组格式
- `TestLoad_WatchDirsTakesPrecedence` — 优先级
- `TestValidate_OverlappingDirs` / `Reversed` — 重叠检测
- `TestValidate_SimilarPrefixNotOverlap` — `/mnt/c` vs `/mnt/cdrom`

更新 `configs/config.yaml` 和 `configs/config.example.yaml`

**测试: 17/17 PASS**

#### Task 2: WSL detection

新建 `internal/watcher/wsl.go`:
- `IsWSL() bool` — 读 `/proc/version` 检测 `microsoft`/`WSL`，`sync.Once` 缓存结果
- `NeedsPollWatcher(path string) bool` — `IsWSL() && strings.HasPrefix(path, "/mnt/")`

新建 `internal/watcher/wsl_test.go`

#### Task 3: Watcher interface extraction

修改 `internal/watcher/watcher.go`:
- 新增 `DirWatcher` interface: `Start(ctx context.Context) error` + `Stop()`
- `Watcher` → `NotifyWatcher`
- `NewWatcher(cfg, processFn)` → `NewNotifyWatcher(dir, processFn)`
- 构造函数接收 `dir string` 而非 `*config.Config`

更新 `watcher_test.go` 中所有引用

#### Task 4: Poll watcher implementation

新建 `internal/watcher/poll.go`:
- `PollWatcher` struct 实现 `DirWatcher`
- `fileState{ModTime, Size}` 快照类型
- `scan()` — `filepath.WalkDir` + 后缀过滤 + `~$` 排除
- `diff()` — 当前 vs 上一轮快照比对，产出 `[]pollEvent`
- `Start(ctx)` — 10s ticker 循环: scan → diff → 更新快照 → Debouncer → WorkerPool
- `Stop()` — cancel + 等待退出
- `BuildSnapshot(dir)` — 供 reconcile 后初始化使用
- `NewPollWatcher(dir, processFn, initialSnapshot)` — 支持预置快照

新建 `internal/watcher/poll_test.go`:
- `TestPollWatcher_Scan` — 验证后缀过滤和 `~$` 排除
- `TestPollWatcher_Diff_Added/Modified/Deleted/NoChanges`
- `TestPollWatcher_Lifecycle` — 端到端验证
- `TestPollWatcher_InitialSnapshot` — 预置快照不产生重复事件
- `TestBuildSnapshot`

#### Task 5: Temp file exclusion

修改 `internal/cleaning/filter.go`:
- `IsSupportedFile()` 新增 `~$` 前缀检查

更新 `filter_test.go` 新增 3 个 `~$` 文件用例

#### Task 6: Reconciliation multi-directory support

修改 `internal/store/reconcile.go`:
- `Reconcile(watchDir string, ...)` → `Reconcile(watchDirs []string, ...)`
- 外层循环遍历所有目录
- 新增 `~$` 临时文件跳过

修改 `persistence_test.go` 调用签名

#### Task 7: Main startup flow integration

重写 `cmd/dense-rag/main.go`:
- 循环 `cfg.WatchDirs` 创建目录
- 统一 `st.Reconcile(cfg.WatchDirs, ...)`
- 共享 `processFn`
- 按目录判断: `NeedsPollWatcher(dir)` → `PollWatcher` (带初始快照) / `NotifyWatcher`
- 所有 watcher 独立 goroutine 启动
- 关闭时循环 `Stop()` 所有 watcher
- 日志显示 `watching [dir1, dir2]` 和每目录 watcher 类型

#### Task 8: Build verification

```
$ go build ./...       # 成功
$ go test ./internal/... -count=1   # 68/68 PASS
```

| 包 | 测试数 | 结果 |
|---|--------|------|
| internal/api | 6 | PASS |
| internal/cleaning | 16 | PASS |
| internal/config | 17 | PASS |
| internal/embedding | 8 | PASS |
| internal/store | 15 | PASS |
| internal/watcher | 21 | PASS (含 poll + wsl 新测试) |

---

## 第六轮：集成测试验证

**用户**: 好，跑一下自动化测试，我要确保原有功能不受影响

**执行**: `go test ./test/ -v -count=1 -timeout 300s`

### 集成测试结果: 17/17 PASS

| 测试 | 耗时 | 结果 |
|------|------|------|
| TC-01 新文件可查询 | 0.72s | PASS (score=0.7377) |
| TC-02 文件路径正确 | 0.02s | PASS |
| TC-03 修改后内容更新 | 0.57s | PASS (score=0.5914) |
| TC-04 删除后不可查询 | 0.56s | PASS |
| TC-05 空目录健康检查 | 0.25s | PASS |
| TC-06 健康计数随文件变化 | 1.87s | PASS |
| TC-07 子目录文件索引 | 1.34s | PASS (score=0.7736) |
| TC-08 动态创建子目录 | 1.83s | PASS (score=0.7997) |
| TC-09 TopK 限制 | 0.80s | PASS (5 results) |
| TC-10 空文件处理 | 2.25s | PASS |
| TC-11 大文件忽略 | 7.26s | PASS |
| TC-12 不支持格式忽略 | 2.25s | PASS |
| TC-13 空查询返回 400 | 0.22s | PASS |
| TC-14 非法 JSON 返回 400 | 0.24s | PASS |
| TC-15 空索引返回空数组 | 0.29s | PASS |
| TC-16 快速修改防抖 | 3.48s | PASS (score=0.7213) |
| TC-17 文件重命名 | 1.38s | PASS (score=0.8063) |

集成测试使用旧格式 `watch_dir: "..."` 配置，验证了向后兼容性。

---

## 变更文件清单

### 新增文件 (4)

| 文件 | 用途 |
|------|------|
| `internal/watcher/wsl.go` | WSL 环境检测 |
| `internal/watcher/wsl_test.go` | WSL 检测测试 |
| `internal/watcher/poll.go` | Poll-based 文件监控 |
| `internal/watcher/poll_test.go` | Poll watcher 测试 |

### 修改文件 (10)

| 文件 | 变更 |
|------|------|
| `internal/config/config.go` | WatchDirs 多目录、rawConfig 双格式、重叠检测 |
| `internal/config/config_test.go` | 新格式/旧格式/重叠检测测试 |
| `internal/watcher/watcher.go` | DirWatcher 接口、Watcher → NotifyWatcher |
| `internal/watcher/watcher_test.go` | 适配 NotifyWatcher |
| `internal/cleaning/filter.go` | ~$ 临时文件排除 |
| `internal/cleaning/filter_test.go` | ~$ 测试用例 |
| `internal/store/reconcile.go` | 多目录 Reconcile |
| `internal/store/persistence_test.go` | 适配 Reconcile 签名 |
| `cmd/dense-rag/main.go` | 多目录 reconcile + 多 watcher 生命周期 |
| `configs/config.yaml` + `config.example.yaml` | watch_dirs 数组格式 |

### OpenSpec 文件 (4)

| 文件 | 用途 |
|------|------|
| `openspec/changes/add-wsl-poll-watcher/proposal.md` | 变更提案 |
| `openspec/changes/add-wsl-poll-watcher/design.md` | 架构决策 |
| `openspec/changes/add-wsl-poll-watcher/tasks.md` | 实施清单 |
| `openspec/changes/add-wsl-poll-watcher/specs/*/spec.md` | 3 个能力域规格 |

---

## 待手动验证

- [ ] 8.1 WSL2 环境 + `/mnt/c/` 目录 poll watcher 增删改检测
- [ ] 8.3 混合配置（原生 + `/mnt/`）同时运行
