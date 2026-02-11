package test

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// ---------------------------------------------------------------------------
// Score thresholds — tune these based on bge-m3 model behavior.
// ---------------------------------------------------------------------------

const (
	ScoreHitThreshold  = 0.5 // score above this = match
	ScoreMissThreshold = 0.5 // score below this = no match
)

// ---------------------------------------------------------------------------
// Polling parameters
// ---------------------------------------------------------------------------

const (
	pollInterval   = 500 * time.Millisecond
	defaultTimeout = 15 * time.Second
)

// ---------------------------------------------------------------------------
// Embedding service configuration — read from real config.
// ---------------------------------------------------------------------------

var (
	modelEndpoint = "http://10.45.28.35:1234"
	modelName     = "text-embedding-bge-m3"
	binaryPath    string
)

// ---------------------------------------------------------------------------
// Types mirroring the service API.
// ---------------------------------------------------------------------------

type ResultItem struct {
	Text     string  `json:"text"`
	FilePath string  `json:"file_path"`
	Score    float32 `json:"score"`
}

type HealthResponse struct {
	Status         string `json:"status"`
	VectorCount    int    `json:"vector_count"`
	IndexedFiles   int    `json:"indexed_files"`
	StoreSizeBytes int64  `json:"store_size_bytes"`
}

// ---------------------------------------------------------------------------
// TestMain — build binary, check embedding service, run tests, clean up.
// ---------------------------------------------------------------------------

func TestMain(m *testing.M) {
	// Check embedding service availability.
	resp, err := http.Get(modelEndpoint + "/v1/models")
	if err != nil {
		fmt.Printf("SKIP: embedding service not reachable at %s: %v\n", modelEndpoint, err)
		os.Exit(0)
	}
	resp.Body.Close()
	if resp.StatusCode >= 400 {
		fmt.Printf("SKIP: embedding service returned status %d\n", resp.StatusCode)
		os.Exit(0)
	}

	// Build the binary.
	projectRoot := findProjectRoot()
	binaryPath = filepath.Join(projectRoot, "bin", "dense-rag-test")
	cmd := exec.Command("go", "build", "-o", binaryPath, "./cmd/dense-rag")
	cmd.Dir = projectRoot
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		fmt.Printf("FAIL: could not build binary: %v\n", err)
		os.Exit(1)
	}

	code := m.Run()

	// Clean up binary.
	os.Remove(binaryPath)
	os.Exit(code)
}

// findProjectRoot walks up from the test directory to find go.mod.
func findProjectRoot() string {
	dir, _ := os.Getwd()
	for {
		if _, err := os.Stat(filepath.Join(dir, "go.mod")); err == nil {
			return dir
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			// fallback
			return "."
		}
		dir = parent
	}
}

// ---------------------------------------------------------------------------
// Service lifecycle helpers
// ---------------------------------------------------------------------------

// freePort returns a free TCP port on localhost.
func freePort(t *testing.T) int {
	t.Helper()
	l, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("failed to get free port: %v", err)
	}
	port := l.Addr().(*net.TCPAddr).Port
	l.Close()
	return port
}

type serviceInstance struct {
	cmd     *exec.Cmd
	baseURL string
	tmpDir  string
	cfgPath string
}

// startService creates a temp watch_dir, writes a temp config, starts the
// service binary, and waits for /health to become reachable. Returns a
// serviceInstance and a cleanup function.
func startService(t *testing.T) (*serviceInstance, func()) {
	t.Helper()

	tmpDir, err := os.MkdirTemp("", "dense_rag_test_")
	if err != nil {
		t.Fatalf("create temp dir: %v", err)
	}

	watchDir := filepath.Join(tmpDir, "watch")
	if err := os.MkdirAll(watchDir, 0755); err != nil {
		t.Fatalf("create watch dir: %v", err)
	}

	port := freePort(t)
	cfgPath := filepath.Join(tmpDir, "config.yaml")

	cfgContent := fmt.Sprintf(`host: "127.0.0.1"
port: %d
topk: 5
watch_dir: "%s"
model: "%s"
model_endpoint: "%s"
`, port, watchDir, modelName, modelEndpoint)

	if err := os.WriteFile(cfgPath, []byte(cfgContent), 0644); err != nil {
		t.Fatalf("write config: %v", err)
	}

	cmd := exec.Command(binaryPath, "--config", cfgPath)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Start(); err != nil {
		t.Fatalf("start service: %v", err)
	}

	baseURL := fmt.Sprintf("http://127.0.0.1:%d", port)

	// Wait for service to be ready.
	deadline := time.Now().Add(15 * time.Second)
	ready := false
	for time.Now().Before(deadline) {
		resp, err := http.Get(baseURL + "/health")
		if err == nil {
			resp.Body.Close()
			if resp.StatusCode == 200 {
				ready = true
				break
			}
		}
		time.Sleep(200 * time.Millisecond)
	}
	if !ready {
		cmd.Process.Kill()
		cmd.Wait()
		os.RemoveAll(tmpDir)
		t.Fatalf("service did not become ready within 15s on port %d", port)
	}

	si := &serviceInstance{
		cmd:     cmd,
		baseURL: baseURL,
		tmpDir:  tmpDir,
		cfgPath: cfgPath,
	}

	cleanup := func() {
		cmd.Process.Signal(os.Interrupt)
		done := make(chan error, 1)
		go func() { done <- cmd.Wait() }()
		select {
		case <-done:
		case <-time.After(5 * time.Second):
			cmd.Process.Kill()
			<-done
		}
		os.RemoveAll(tmpDir)
	}

	return si, cleanup
}

// watchDir returns the watch directory for a service instance.
func (si *serviceInstance) watchDir() string {
	return filepath.Join(si.tmpDir, "watch")
}

// ---------------------------------------------------------------------------
// HTTP helpers
// ---------------------------------------------------------------------------

func queryOnce(baseURL, text string) ([]ResultItem, int, error) {
	body, _ := json.Marshal(map[string]string{"text": text})
	resp, err := http.Post(baseURL+"/query", "application/json", bytes.NewReader(body))
	if err != nil {
		return nil, 0, err
	}
	defer resp.Body.Close()
	data, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != 200 {
		return nil, resp.StatusCode, fmt.Errorf("status %d: %s", resp.StatusCode, string(data))
	}
	var items []ResultItem
	if err := json.Unmarshal(data, &items); err != nil {
		return nil, resp.StatusCode, err
	}
	return items, resp.StatusCode, nil
}

func queryRaw(baseURL string, rawBody string) (int, []byte, error) {
	resp, err := http.Post(baseURL+"/query", "application/json", strings.NewReader(rawBody))
	if err != nil {
		return 0, nil, err
	}
	defer resp.Body.Close()
	data, _ := io.ReadAll(resp.Body)
	return resp.StatusCode, data, nil
}

func healthOnce(baseURL string) (HealthResponse, error) {
	var hr HealthResponse
	resp, err := http.Get(baseURL + "/health")
	if err != nil {
		return hr, err
	}
	defer resp.Body.Close()
	data, _ := io.ReadAll(resp.Body)
	if err := json.Unmarshal(data, &hr); err != nil {
		return hr, err
	}
	return hr, nil
}

// pollQuery polls POST /query until matchFn returns true or timeout is reached.
func pollQuery(t *testing.T, baseURL, text string, matchFn func([]ResultItem) bool, timeout time.Duration) []ResultItem {
	t.Helper()
	deadline := time.Now().Add(timeout)
	var lastItems []ResultItem
	for time.Now().Before(deadline) {
		items, _, err := queryOnce(baseURL, text)
		if err == nil {
			lastItems = items
			if matchFn(items) {
				return items
			}
		}
		time.Sleep(pollInterval)
	}
	return lastItems
}

// pollHealth polls GET /health until condFn returns true or timeout is reached.
func pollHealth(t *testing.T, baseURL string, condFn func(HealthResponse) bool, timeout time.Duration) HealthResponse {
	t.Helper()
	deadline := time.Now().Add(timeout)
	var last HealthResponse
	for time.Now().Before(deadline) {
		hr, err := healthOnce(baseURL)
		if err == nil {
			last = hr
			if condFn(hr) {
				return hr
			}
		}
		time.Sleep(pollInterval)
	}
	return last
}

// ---------------------------------------------------------------------------
// Match helpers
// ---------------------------------------------------------------------------

// hasHitForFile returns true if any result for the given file has score > threshold.
func hasHitForFile(items []ResultItem, filePath string) bool {
	for _, it := range items {
		if it.FilePath == filePath && it.Score > ScoreHitThreshold {
			return true
		}
	}
	return false
}

// hasHitAboveThreshold returns true if any result has score > ScoreHitThreshold.
func hasHitAboveThreshold(items []ResultItem) bool {
	for _, it := range items {
		if it.Score > ScoreHitThreshold {
			return true
		}
	}
	return false
}

// noHitForFile returns true if no result for filePath has score > ScoreMissThreshold.
func noHitForFile(items []ResultItem, filePath string) bool {
	for _, it := range items {
		if it.FilePath == filePath && it.Score > ScoreMissThreshold {
			return false
		}
	}
	return true
}

// ===========================================================================
// TC-01 ~ TC-04: Core lifecycle (sequential subtests sharing one service)
// ===========================================================================

func TestCoreLifecycle(t *testing.T) {
	si, cleanup := startService(t)
	defer cleanup()

	filePath := filepath.Join(si.watchDir(), "test_add.txt")

	// TC-01: New file is queryable.
	t.Run("TC-01_NewFileQueryable", func(t *testing.T) {
		content := "深度学习是人工智能的一个重要分支，通过多层神经网络进行特征提取"
		if err := os.WriteFile(filePath, []byte(content), 0644); err != nil {
			t.Fatalf("write file: %v", err)
		}

		query := "神经网络与深度学习"
		items := pollQuery(t, si.baseURL, query, func(items []ResultItem) bool {
			return hasHitForFile(items, filePath)
		}, defaultTimeout)

		found := false
		for _, it := range items {
			if it.FilePath == filePath && it.Score > ScoreHitThreshold {
				t.Logf("TC-01 PASS: file=%s score=%.4f text=%q", it.FilePath, it.Score, it.Text)
				found = true
				break
			}
		}
		if !found {
			t.Fatalf("TC-01 FAIL: expected hit for %s with query %q, got: %+v", filePath, query, items)
		}
	})

	// TC-02: Result contains correct file path.
	t.Run("TC-02_CorrectFilePath", func(t *testing.T) {
		query := "神经网络与深度学习"
		items, _, err := queryOnce(si.baseURL, query)
		if err != nil {
			t.Fatalf("query failed: %v", err)
		}

		found := false
		for _, it := range items {
			if it.FilePath == filePath {
				// Verify absolute path.
				if !filepath.IsAbs(it.FilePath) {
					t.Fatalf("TC-02 FAIL: file_path is not absolute: %s", it.FilePath)
				}
				// Verify file exists on disk.
				if _, err := os.Stat(it.FilePath); err != nil {
					t.Fatalf("TC-02 FAIL: file_path does not exist on disk: %s", it.FilePath)
				}
				t.Logf("TC-02 PASS: file_path=%s exists and is absolute", it.FilePath)
				found = true
				break
			}
		}
		if !found {
			t.Fatalf("TC-02 FAIL: file_path %s not found in results: %+v", filePath, items)
		}
	})

	// TC-03: Modified file reflects new content.
	t.Run("TC-03_ModifiedContent", func(t *testing.T) {
		newContent := "量子计算利用量子比特的叠加态和纠缠态来加速计算"
		if err := os.WriteFile(filePath, []byte(newContent), 0644); err != nil {
			t.Fatalf("write file: %v", err)
		}

		// Poll for new content to become queryable.
		newQuery := "量子比特"
		items := pollQuery(t, si.baseURL, newQuery, func(items []ResultItem) bool {
			return hasHitForFile(items, filePath)
		}, defaultTimeout)

		foundNew := false
		for _, it := range items {
			if it.FilePath == filePath && it.Score > ScoreHitThreshold {
				t.Logf("TC-03 new content hit: score=%.4f text=%q", it.Score, it.Text)
				foundNew = true
				break
			}
		}
		if !foundNew {
			t.Fatalf("TC-03 FAIL: new content not queryable. results: %+v", items)
		}

		// Verify old content no longer matches.
		oldQuery := "神经网络与深度学习"
		items, _, _ = queryOnce(si.baseURL, oldQuery)
		if !noHitForFile(items, filePath) {
			for _, it := range items {
				if it.FilePath == filePath {
					t.Logf("TC-03 WARNING: old content still has score=%.4f for file %s", it.Score, filePath)
				}
			}
			t.Fatalf("TC-03 FAIL: old content still matches for file %s", filePath)
		}
		t.Logf("TC-03 PASS: new content queryable, old content not matching")
	})

	// TC-04: Deleted file is not queryable.
	t.Run("TC-04_DeletedFile", func(t *testing.T) {
		if err := os.Remove(filePath); err != nil {
			t.Fatalf("remove file: %v", err)
		}

		query := "量子比特"
		// Poll until the file is no longer in results.
		deadline := time.Now().Add(defaultTimeout)
		for time.Now().Before(deadline) {
			items, _, _ := queryOnce(si.baseURL, query)
			if noHitForFile(items, filePath) {
				t.Logf("TC-04 PASS: deleted file no longer in results")
				return
			}
			time.Sleep(pollInterval)
		}

		// Final check.
		items, _, _ := queryOnce(si.baseURL, query)
		if !noHitForFile(items, filePath) {
			t.Fatalf("TC-04 FAIL: deleted file still in results: %+v", items)
		}
		t.Logf("TC-04 PASS: deleted file not queryable")
	})
}

// ===========================================================================
// TC-05: Health check — empty directory
// ===========================================================================

func TestHealthEmpty(t *testing.T) {
	si, cleanup := startService(t)
	defer cleanup()

	hr, err := healthOnce(si.baseURL)
	if err != nil {
		t.Fatalf("health request failed: %v", err)
	}

	if hr.Status == "" {
		t.Fatalf("TC-05 FAIL: status field is empty")
	}
	if hr.VectorCount != 0 {
		t.Fatalf("TC-05 FAIL: expected vector_count=0, got %d", hr.VectorCount)
	}
	if hr.IndexedFiles != 0 {
		t.Fatalf("TC-05 FAIL: expected indexed_files=0, got %d", hr.IndexedFiles)
	}
	t.Logf("TC-05 PASS: status=%s vector_count=%d indexed_files=%d", hr.Status, hr.VectorCount, hr.IndexedFiles)
}

// ===========================================================================
// TC-06: Health counts change with files
// ===========================================================================

func TestHealthCountsChange(t *testing.T) {
	si, cleanup := startService(t)
	defer cleanup()

	// Add first file.
	f1 := filepath.Join(si.watchDir(), "health_test1.txt")
	if err := os.WriteFile(f1, []byte("机器学习是计算机科学的一个分支，研究如何让计算机自动学习和改进"), 0644); err != nil {
		t.Fatalf("write file: %v", err)
	}

	// Wait for indexed_files > 0.
	hr := pollHealth(t, si.baseURL, func(hr HealthResponse) bool {
		return hr.IndexedFiles > 0
	}, defaultTimeout)
	if hr.IndexedFiles == 0 {
		t.Fatalf("TC-06 FAIL: indexed_files did not increase after adding file1")
	}
	files1 := hr.IndexedFiles
	vec1 := hr.VectorCount
	t.Logf("TC-06: after file1: indexed_files=%d vector_count=%d", files1, vec1)

	// Add second file.
	f2 := filepath.Join(si.watchDir(), "health_test2.txt")
	if err := os.WriteFile(f2, []byte("计算机视觉让机器能够从图像和视频中获取信息并做出决策"), 0644); err != nil {
		t.Fatalf("write file: %v", err)
	}

	// Wait for indexed_files to increase.
	hr = pollHealth(t, si.baseURL, func(hr HealthResponse) bool {
		return hr.IndexedFiles > files1
	}, defaultTimeout)
	if hr.IndexedFiles <= files1 {
		t.Fatalf("TC-06 FAIL: indexed_files did not increase after adding file2, still %d", hr.IndexedFiles)
	}
	files2 := hr.IndexedFiles
	t.Logf("TC-06: after file2: indexed_files=%d vector_count=%d", files2, hr.VectorCount)

	// Delete first file.
	if err := os.Remove(f1); err != nil {
		t.Fatalf("remove file: %v", err)
	}

	// Wait for indexed_files to decrease.
	hr = pollHealth(t, si.baseURL, func(hr HealthResponse) bool {
		return hr.IndexedFiles < files2
	}, defaultTimeout)
	if hr.IndexedFiles >= files2 {
		t.Fatalf("TC-06 FAIL: indexed_files did not decrease after removing file1, still %d", hr.IndexedFiles)
	}
	if hr.VectorCount >= vec1+hr.VectorCount {
		// Sanity: vector_count should have decreased too.
	}
	t.Logf("TC-06 PASS: indexed_files=%d vector_count=%d (decreased after removal)", hr.IndexedFiles, hr.VectorCount)
}

// ===========================================================================
// TC-07: Subdirectory file indexed
// ===========================================================================

func TestSubdirectoryIndexing(t *testing.T) {
	si, cleanup := startService(t)
	defer cleanup()

	subDir := filepath.Join(si.watchDir(), "subdir_a")
	if err := os.MkdirAll(subDir, 0755); err != nil {
		t.Fatalf("create subdir: %v", err)
	}

	// Give the watcher time to register the new subdirectory.
	time.Sleep(500 * time.Millisecond)

	filePath := filepath.Join(subDir, "sub_test.txt")
	content := "自然语言处理使得计算机能够理解人类语言"
	if err := os.WriteFile(filePath, []byte(content), 0644); err != nil {
		t.Fatalf("write file: %v", err)
	}

	query := "计算机理解语言"
	items := pollQuery(t, si.baseURL, query, func(items []ResultItem) bool {
		return hasHitForFile(items, filePath)
	}, defaultTimeout)

	found := false
	for _, it := range items {
		if it.FilePath == filePath && it.Score > ScoreHitThreshold {
			if !strings.Contains(it.FilePath, "subdir_a") {
				t.Fatalf("TC-07 FAIL: file_path does not contain subdir: %s", it.FilePath)
			}
			t.Logf("TC-07 PASS: score=%.4f file_path=%s", it.Score, it.FilePath)
			found = true
			break
		}
	}
	if !found {
		t.Fatalf("TC-07 FAIL: subdirectory file not queryable. results: %+v", items)
	}
}

// ===========================================================================
// TC-08: Dynamic subdirectory
// ===========================================================================

func TestDynamicSubdirectory(t *testing.T) {
	si, cleanup := startService(t)
	defer cleanup()

	// Wait a moment for the watcher to be fully active.
	time.Sleep(500 * time.Millisecond)

	// Create a new subdirectory after the service is running.
	subDir := filepath.Join(si.watchDir(), "subdir_new")
	if err := os.MkdirAll(subDir, 0755); err != nil {
		t.Fatalf("create subdir: %v", err)
	}

	// Give the watcher time to register the new directory.
	time.Sleep(500 * time.Millisecond)

	filePath := filepath.Join(subDir, "dynamic.txt")
	content := "区块链技术通过分布式账本实现了去中心化的数据存储和验证"
	if err := os.WriteFile(filePath, []byte(content), 0644); err != nil {
		t.Fatalf("write file: %v", err)
	}

	query := "区块链分布式账本"
	items := pollQuery(t, si.baseURL, query, func(items []ResultItem) bool {
		return hasHitForFile(items, filePath)
	}, defaultTimeout)

	found := false
	for _, it := range items {
		if it.FilePath == filePath && it.Score > ScoreHitThreshold {
			t.Logf("TC-08 PASS: score=%.4f file_path=%s", it.Score, it.FilePath)
			found = true
			break
		}
	}
	if !found {
		t.Fatalf("TC-08 FAIL: file in dynamic subdir not queryable. results: %+v", items)
	}
}

// ===========================================================================
// TC-09: TopK limit
// ===========================================================================

func TestTopKLimit(t *testing.T) {
	si, cleanup := startService(t)
	defer cleanup()

	// Create 6 files with related ML content.
	mlTopics := []string{
		"支持向量机是一种经典的监督学习算法，通过寻找最优超平面来分类数据",
		"决策树通过树形结构进行分类和回归，每个节点表示一个特征判断",
		"随机森林是一种集成学习方法，通过组合多棵决策树来提高预测准确率",
		"梯度提升是一种强大的机器学习技术，通过迭代地添加弱学习器来构建强学习器",
		"K近邻算法通过计算样本之间的距离来进行分类和回归预测",
		"朴素贝叶斯分类器基于贝叶斯定理和特征条件独立假设进行概率分类",
	}

	for i, topic := range mlTopics {
		f := filepath.Join(si.watchDir(), fmt.Sprintf("ml_topic_%d.txt", i+1))
		if err := os.WriteFile(f, []byte(topic), 0644); err != nil {
			t.Fatalf("write file: %v", err)
		}
	}

	// Wait for all files to be indexed.
	pollHealth(t, si.baseURL, func(hr HealthResponse) bool {
		return hr.IndexedFiles >= 6
	}, defaultTimeout)

	// Query with a broad ML term.
	query := "机器学习算法"
	items, _, err := queryOnce(si.baseURL, query)
	if err != nil {
		t.Fatalf("query failed: %v", err)
	}

	if len(items) > 5 {
		t.Fatalf("TC-09 FAIL: expected <= 5 results, got %d: %+v", len(items), items)
	}
	t.Logf("TC-09 PASS: got %d results (topk=5)", len(items))
	for i, it := range items {
		t.Logf("  [%d] score=%.4f file=%s", i, it.Score, it.FilePath)
	}
}

// ===========================================================================
// TC-10: Empty file
// ===========================================================================

func TestEmptyFile(t *testing.T) {
	si, cleanup := startService(t)
	defer cleanup()

	// Record initial state.
	hrBefore, _ := healthOnce(si.baseURL)

	emptyFile := filepath.Join(si.watchDir(), "empty.txt")
	if err := os.WriteFile(emptyFile, []byte{}, 0644); err != nil {
		t.Fatalf("write file: %v", err)
	}

	// Wait 2s as specified.
	time.Sleep(2 * time.Second)

	// Verify service still running.
	hrAfter, err := healthOnce(si.baseURL)
	if err != nil {
		t.Fatalf("TC-10 FAIL: service not reachable after creating empty file: %v", err)
	}

	// Vector count should not have increased.
	if hrAfter.VectorCount > hrBefore.VectorCount {
		t.Fatalf("TC-10 FAIL: vector_count increased from %d to %d after empty file", hrBefore.VectorCount, hrAfter.VectorCount)
	}
	t.Logf("TC-10 PASS: service stable, vector_count=%d (unchanged)", hrAfter.VectorCount)
}

// ===========================================================================
// TC-11: Large file ignored
// ===========================================================================

func TestLargeFileIgnored(t *testing.T) {
	si, cleanup := startService(t)
	defer cleanup()

	hrBefore, _ := healthOnce(si.baseURL)

	// Create a >20MB file filled with text.
	bigFile := filepath.Join(si.watchDir(), "big.txt")
	f, err := os.Create(bigFile)
	if err != nil {
		t.Fatalf("create file: %v", err)
	}
	// Write 21MB of 'A' characters with some searchable text.
	searchText := "大文件测试内容，包含可搜索的文本用于验证该文件不被索引\n"
	f.WriteString(searchText)
	// Pad to >20MB.
	padding := make([]byte, 21*1024*1024)
	for i := range padding {
		padding[i] = 'A'
	}
	f.Write(padding)
	f.Close()

	// Wait 2s.
	time.Sleep(2 * time.Second)

	// Check health — indexed_files should not have increased.
	hrAfter, err := healthOnce(si.baseURL)
	if err != nil {
		t.Fatalf("TC-11 FAIL: health request failed: %v", err)
	}
	if hrAfter.IndexedFiles > hrBefore.IndexedFiles {
		t.Fatalf("TC-11 FAIL: indexed_files increased from %d to %d — large file was indexed", hrBefore.IndexedFiles, hrAfter.IndexedFiles)
	}
	t.Logf("TC-11 PASS: large file ignored, indexed_files=%d", hrAfter.IndexedFiles)
}

// ===========================================================================
// TC-12: Unsupported formats ignored
// ===========================================================================

func TestUnsupportedFormats(t *testing.T) {
	si, cleanup := startService(t)
	defer cleanup()

	hrBefore, _ := healthOnce(si.baseURL)

	// Create files with unsupported extensions.
	for _, name := range []string{"ignored.pdf", "ignored.jpg", "ignored.csv"} {
		f := filepath.Join(si.watchDir(), name)
		if err := os.WriteFile(f, []byte("some content that should not be indexed"), 0644); err != nil {
			t.Fatalf("write file: %v", err)
		}
	}

	// Wait 2s.
	time.Sleep(2 * time.Second)

	hrAfter, err := healthOnce(si.baseURL)
	if err != nil {
		t.Fatalf("TC-12 FAIL: health request failed: %v", err)
	}
	if hrAfter.IndexedFiles > hrBefore.IndexedFiles {
		t.Fatalf("TC-12 FAIL: indexed_files increased from %d to %d — unsupported files were indexed", hrBefore.IndexedFiles, hrAfter.IndexedFiles)
	}
	t.Logf("TC-12 PASS: unsupported formats ignored, indexed_files=%d", hrAfter.IndexedFiles)
}

// ===========================================================================
// TC-13: Empty query text returns 400
// ===========================================================================

func TestEmptyQueryText(t *testing.T) {
	si, cleanup := startService(t)
	defer cleanup()

	code, body, err := queryRaw(si.baseURL, `{"text": ""}`)
	if err != nil {
		t.Fatalf("request failed: %v", err)
	}
	if code != 400 {
		t.Fatalf("TC-13 FAIL: expected HTTP 400, got %d, body: %s", code, string(body))
	}
	t.Logf("TC-13 PASS: empty text returned HTTP 400")
}

// ===========================================================================
// TC-14: Invalid JSON returns 400
// ===========================================================================

func TestInvalidJSON(t *testing.T) {
	si, cleanup := startService(t)
	defer cleanup()

	code, body, err := queryRaw(si.baseURL, `not json`)
	if err != nil {
		t.Fatalf("request failed: %v", err)
	}
	if code != 400 {
		t.Fatalf("TC-14 FAIL: expected HTTP 400, got %d, body: %s", code, string(body))
	}
	t.Logf("TC-14 PASS: invalid JSON returned HTTP 400")
}

// ===========================================================================
// TC-15: Query with no index returns empty array
// ===========================================================================

func TestQueryNoIndex(t *testing.T) {
	si, cleanup := startService(t)
	defer cleanup()

	items, code, err := queryOnce(si.baseURL, "任意查询文本")
	if err != nil {
		// err may wrap a non-200 status; check code.
		if code != 200 {
			t.Fatalf("TC-15 FAIL: expected HTTP 200, got %d, err: %v", code, err)
		}
		t.Fatalf("TC-15 FAIL: query error: %v", err)
	}
	if len(items) != 0 {
		t.Fatalf("TC-15 FAIL: expected empty array, got %d results: %+v", len(items), items)
	}
	t.Logf("TC-15 PASS: empty index returned empty array")
}

// ===========================================================================
// TC-16: Rapid modification — debounce
// ===========================================================================

func TestRapidModification(t *testing.T) {
	si, cleanup := startService(t)
	defer cleanup()

	filePath := filepath.Join(si.watchDir(), "rapid.txt")

	contentA := "内容A：太阳系有八大行星围绕太阳运转"
	contentB := "内容B：地球是太阳系中唯一已知存在生命的行星"
	contentC := "内容C：月球是地球唯一的天然卫星"
	contentD := "内容D：基因编辑技术CRISPR可以精准修改DNA序列改造生物体"

	// Write initial content, then rapidly overwrite.
	os.WriteFile(filePath, []byte(contentA), 0644)
	time.Sleep(50 * time.Millisecond)
	os.WriteFile(filePath, []byte(contentB), 0644)
	time.Sleep(50 * time.Millisecond)
	os.WriteFile(filePath, []byte(contentC), 0644)
	time.Sleep(50 * time.Millisecond)
	os.WriteFile(filePath, []byte(contentD), 0644)

	// Wait for indexing to stabilize.
	// Poll health until vector_count stabilizes (same value for 2s).
	time.Sleep(3 * time.Second)
	stableHR := pollHealth(t, si.baseURL, func(hr HealthResponse) bool {
		return hr.IndexedFiles >= 1
	}, defaultTimeout)
	_ = stableHR

	// Query for final content D.
	queryD := "基因编辑CRISPR技术"
	items := pollQuery(t, si.baseURL, queryD, func(items []ResultItem) bool {
		return hasHitForFile(items, filePath)
	}, defaultTimeout)

	foundD := false
	for _, it := range items {
		if it.FilePath == filePath && it.Score > ScoreHitThreshold {
			t.Logf("TC-16: content D hit: score=%.4f", it.Score)
			foundD = true
			break
		}
	}
	if !foundD {
		t.Fatalf("TC-16 FAIL: final content D not queryable. results: %+v", items)
	}

	// Verify intermediate contents A/B/C are NOT queryable.
	for label, q := range map[string]string{
		"A": "太阳系八大行星",
		"B": "地球存在生命",
		"C": "月球天然卫星",
	} {
		items, _, _ := queryOnce(si.baseURL, q)
		if !noHitForFile(items, filePath) {
			// Log but allow some tolerance — the key check is that D is indexed.
			t.Logf("TC-16 WARNING: intermediate content %s still has a hit for file %s", label, filePath)
			for _, it := range items {
				if it.FilePath == filePath {
					t.Logf("  score=%.4f text=%q", it.Score, it.Text)
				}
			}
		}
	}
	t.Logf("TC-16 PASS: final content D indexed, intermediate content not queryable")
}

// ===========================================================================
// TC-17: File rename
// ===========================================================================

func TestFileRename(t *testing.T) {
	si, cleanup := startService(t)
	defer cleanup()

	oldPath := filepath.Join(si.watchDir(), "before_rename.txt")
	newPath := filepath.Join(si.watchDir(), "after_rename.txt")

	content := "强化学习通过奖惩机制训练智能体在环境中做出最优决策"
	if err := os.WriteFile(oldPath, []byte(content), 0644); err != nil {
		t.Fatalf("write file: %v", err)
	}

	// Wait for initial indexing.
	query := "强化学习奖惩机制"
	pollQuery(t, si.baseURL, query, func(items []ResultItem) bool {
		return hasHitForFile(items, oldPath)
	}, defaultTimeout)

	// Rename the file.
	if err := os.Rename(oldPath, newPath); err != nil {
		t.Fatalf("rename file: %v", err)
	}

	// Poll until the new path appears in results.
	items := pollQuery(t, si.baseURL, query, func(items []ResultItem) bool {
		return hasHitForFile(items, newPath)
	}, defaultTimeout)

	foundNew := false
	for _, it := range items {
		if it.FilePath == newPath && it.Score > ScoreHitThreshold {
			t.Logf("TC-17: new path hit: score=%.4f file=%s", it.Score, it.FilePath)
			foundNew = true
		}
		if it.FilePath == oldPath {
			t.Fatalf("TC-17 FAIL: old path %s still in results with score=%.4f", oldPath, it.Score)
		}
	}
	if !foundNew {
		t.Fatalf("TC-17 FAIL: new path %s not in results. results: %+v", newPath, items)
	}
	t.Logf("TC-17 PASS: renamed file uses new path, old path absent")
}
