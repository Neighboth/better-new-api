package byog

import (
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
	"github.com/gin-gonic/gin"
)

// ModelStore hands the model files to the workers.
//
// The device downloads the model from the coordinator instead of from the model host.
// The coordinator has a stable connection, keeps the file in a cache and therefore
// every worker of an installation fetches the model over the same fast path.
type ModelStore struct {
	mu   sync.Mutex
	dir  string
	jobs map[string]*modelJob
}

// modelJob is a running download of a model file.
type modelJob struct {
	modelId    string
	downloaded int64
	total      int64
	started    time.Time
	err        error
	running    bool
}

// GlobalModelStore serves the model files of the installation.
var GlobalModelStore = NewModelStore("")

// NewModelStore creates a model store that keeps the files in the given directory.
func NewModelStore(directory string) *ModelStore {
	if directory == "" {
		directory = os.Getenv("BYOG_MODEL_DIR")
	}
	if directory == "" {
		directory = filepath.Join(modelDataDirectory(), "byog_models")
	}
	return &ModelStore{dir: directory, jobs: make(map[string]*modelJob)}
}

// modelDataDirectory returns the directory the database lives in.
func modelDataDirectory() string {
	path := common.SQLitePath
	if index := strings.Index(path, "?"); index >= 0 {
		path = path[:index]
	}
	if path == "" || path == ":memory:" {
		return "."
	}
	return filepath.Dir(path)
}

// Directory returns the directory of the cached model files.
func (s *ModelStore) Directory() string {
	return s.dir
}

// pathFor returns the path of a cached model file.
func (s *ModelStore) pathFor(modelId string) string {
	safe := strings.NewReplacer("/", "_", "\\", "_", "..", "_").Replace(modelId)
	return filepath.Join(s.dir, safe+".gguf")
}

// job returns the running download of a model.
func (s *ModelStore) job(modelId string) *modelJob {
	s.mu.Lock()
	defer s.mu.Unlock()
	job, ok := s.jobs[modelId]
	if !ok {
		job = &modelJob{modelId: modelId, started: time.Now()}
		s.jobs[modelId] = job
	}
	return job
}

// Progress returns the download progress of a model for the panel.
func (s *ModelStore) Progress(modelId string) (downloaded int64, total int64, running bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	job, ok := s.jobs[modelId]
	if !ok {
		return 0, 0, false
	}
	return job.downloaded, job.total, job.err == nil && job.downloaded < job.total
}

// ServeModelFile streams a model file to a worker.
//
// The file is taken from the cache when it is complete, otherwise the download from
// the model host is streamed to the worker and written into the cache at the same
// time. A request for a part of the model waits for the complete file in the cache,
// the transfer of the model host is never started twice.
func (s *ModelStore) ServeModelFile(c *gin.Context, modelId string) {
	modelDef, err := model.GetByogModelByModelId(modelId)
	if err != nil || modelDef == nil || strings.TrimSpace(modelDef.DownloadUrl) == "" {
		c.JSON(http.StatusNotFound, gin.H{"success": false, "message": "model not found"})
		return
	}

	if err := os.MkdirAll(s.dir, 0o755); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"success": false, "message": err.Error()})
		return
	}

	target := s.pathFor(modelId)
	if info, err := os.Stat(target); err == nil && info.Size() > 0 {
		serveModelFile(c, target, info.Size())
		return
	}

	// A request for a part of the model is served from the cache that the
	// transfer of the model host fills. The transfer is never started twice.
	if c.GetHeader("Range") != "" {
		if s.startDownload(modelId) {
			go s.downloadModel(modelId, modelDef, target)
		}
		s.serveModelRange(c, modelId, target, modelDef.FileSize)
		return
	}

	// The model is downloaded at the moment. The transfer of the model host is
	// shared, the request waits for the cache and is served from it.
	if s.isDownloading(modelId) {
		if !s.waitForCompletedModel(modelId, target) {
			c.JSON(http.StatusGatewayTimeout, gin.H{"success": false, "message": "the model is still being downloaded"})
			return
		}
		info, err := os.Stat(target)
		if err != nil || info.Size() == 0 {
			c.JSON(http.StatusBadGateway, gin.H{"success": false, "message": "the model could not be cached"})
			return
		}
		serveModelFile(c, target, info.Size())
		return
	}

	// The whole model is streamed to the worker and cached at the same time.
	s.streamModelToWorker(c, modelId, modelDef, target)
}

// serveModelRange serves a part of a model from the cache that the running
// transfer fills.
//
// The worker receives the bytes of its range while the model is downloaded, the
// transfer of the model host is shared by all the workers of the installation.
func (s *ModelStore) serveModelRange(c *gin.Context, modelId string, target string, total int64) {
	_, end, ok := parseRange(c.GetHeader("Range"), total)
	if !ok {
		c.JSON(http.StatusRequestedRangeNotSatisfiable, gin.H{"success": false, "message": "the range is not valid"})
		return
	}

	partPath := target + ".part"
	deadline := time.Now().Add(2 * time.Hour)
	for time.Now().Before(deadline) {
		if info, err := os.Stat(target); err == nil && info.Size() > 0 {
			// The transfer is complete, the complete file is served.
			serveModelFile(c, target, info.Size())
			return
		}
		if info, err := os.Stat(partPath); err == nil && info.Size() > end {
			serveModelFile(c, partPath, info.Size())
			return
		}
		s.mu.Lock()
		job, known := s.jobs[modelId]
		failed := known && job.err != nil && !job.running
		s.mu.Unlock()
		if failed {
			c.JSON(http.StatusBadGateway, gin.H{"success": false, "message": "the model could not be downloaded"})
			return
		}
		time.Sleep(200 * time.Millisecond)
	}
	c.JSON(http.StatusGatewayTimeout, gin.H{"success": false, "message": "the model is still being downloaded"})
}

// rememberModelSize writes the real size of a model into the database.
//
// The size of a model is published with the model host. A worker that receives a
// wrong size would download a part of the model only.
func (s *ModelStore) rememberModelSize(modelId string, modelDef *model.ByogModel, size int64) {
	if size <= 0 || modelDef == nil {
		return
	}
	if modelDef.FileSize == size {
		return
	}
	modelDef.FileSize = size
	_ = model.DB.Model(&model.ByogModel{}).
		Where("model_id = ?", modelId).
		Update("file_size", size).Error
}

// isDownloading reports whether a transfer of the model host runs at the moment.
func (s *ModelStore) isDownloading(modelId string) bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	job, ok := s.jobs[modelId]
	return ok && job.running
}

// waitForCompletedModel waits until the model is completely in the cache.
//
// The wait ends when the transfer of the model host failed or when it took longer
// than two hours.
func (s *ModelStore) waitForCompletedModel(modelId string, target string) bool {
	deadline := time.Now().Add(2 * time.Hour)
	for time.Now().Before(deadline) {
		if info, err := os.Stat(target); err == nil && info.Size() > 0 {
			return true
		}
		s.mu.Lock()
		job, ok := s.jobs[modelId]
		failed := ok && job.err != nil && !job.running
		s.mu.Unlock()
		if failed {
			return false
		}
		time.Sleep(250 * time.Millisecond)
	}
	return false
}

// streamModelToWorker streams the model host to the worker and writes the file
// into the cache at the same time.
func (s *ModelStore) streamModelToWorker(c *gin.Context, modelId string, modelDef *model.ByogModel, target string) {
	if !s.startDownload(modelId) {
		// Another request started the transfer of the model host, the cache is
		// used as soon as it is complete.
		if !s.waitForCompletedModel(modelId, target) {
			c.JSON(http.StatusGatewayTimeout, gin.H{"success": false, "message": "the model is still being downloaded"})
			return
		}
		info, err := os.Stat(target)
		if err != nil || info.Size() == 0 {
			c.JSON(http.StatusBadGateway, gin.H{"success": false, "message": "the model could not be cached"})
			return
		}
		serveModelFile(c, target, info.Size())
		return
	}

	upstream, err := http.NewRequest(http.MethodGet, modelDef.DownloadUrl, nil)
	if err != nil {
		s.finishDownload(modelId, err)
		c.JSON(http.StatusBadGateway, gin.H{"success": false, "message": err.Error()})
		return
	}
	upstream.Header.Set("User-Agent", "BYOG-Worker/1.0")

	resp, err := upstreamClient.Do(upstream)
	if err != nil {
		s.finishDownload(modelId, err)
		c.JSON(http.StatusBadGateway, gin.H{"success": false, "message": err.Error()})
		return
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusPartialContent {
		failure := fmt.Errorf("model host returned %d", resp.StatusCode)
		s.finishDownload(modelId, failure)
		c.JSON(http.StatusBadGateway, gin.H{"success": false, "message": failure.Error()})
		return
	}

	job := s.job(modelId)
	s.mu.Lock()
	job.total = resp.ContentLength
	s.mu.Unlock()
	s.rememberModelSize(modelId, modelDef, resp.ContentLength)

	cache, err := os.OpenFile(target+".part", os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0o644)
	if err != nil {
		s.finishDownload(modelId, err)
		c.JSON(http.StatusInternalServerError, gin.H{"success": false, "message": err.Error()})
		return
	}
	defer cache.Close()

	c.Writer.Header().Set("Content-Type", "application/octet-stream")
	c.Writer.Header().Set("Accept-Ranges", "bytes")
	if resp.ContentLength > 0 {
		c.Writer.Header().Set("Content-Length", fmt.Sprintf("%d", resp.ContentLength))
	}
	c.Writer.WriteHeader(http.StatusOK)

	buffer := make([]byte, 256*1024)
	var written int64
	for {
		n, readErr := resp.Body.Read(buffer)
		if n > 0 {
			if _, err := c.Writer.Write(buffer[:n]); err != nil {
				s.finishDownload(modelId, err)
				return
			}
			if _, err := cache.Write(buffer[:n]); err != nil {
				s.finishDownload(modelId, err)
				return
			}
			written += int64(n)
			s.mu.Lock()
			job.downloaded = written
			s.mu.Unlock()
		}
		if readErr != nil {
			if readErr != io.EOF {
				s.finishDownload(modelId, readErr)
				return
			}
			break
		}
	}

	if err := cache.Close(); err != nil {
		s.finishDownload(modelId, err)
		return
	}
	if err := renameWithRetry(target+".part", target); err != nil {
		s.finishDownload(modelId, err)
		return
	}
	s.finishDownload(modelId, nil)
}

// downloadModel fetches a model from the model host into the cache.
//
// The transfer has to be started with startDownload, the function ends it with
// finishDownload. The file is only used when the transfer is complete, a part of a
// model never reaches the cache.
func (s *ModelStore) downloadModel(modelId string, modelDef *model.ByogModel, target string) {
	upstream, err := http.NewRequest(http.MethodGet, modelDef.DownloadUrl, nil)
	if err != nil {
		s.finishDownload(modelId, err)
		return
	}
	upstream.Header.Set("User-Agent", "BYOG-Worker/1.0")

	resp, err := upstreamClient.Do(upstream)
	if err != nil {
		s.finishDownload(modelId, err)
		return
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		s.finishDownload(modelId, fmt.Errorf("model host returned %d", resp.StatusCode))
		return
	}

	job := s.job(modelId)
	s.mu.Lock()
	job.total = resp.ContentLength
	s.mu.Unlock()
	s.rememberModelSize(modelId, modelDef, resp.ContentLength)

	cache, err := os.OpenFile(target+".part", os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0o644)
	if err != nil {
		s.finishDownload(modelId, err)
		return
	}
	defer cache.Close()

	buffer := make([]byte, 256*1024)
	var written int64
	for {
		n, readErr := resp.Body.Read(buffer)
		if n > 0 {
			if _, err := cache.Write(buffer[:n]); err != nil {
				s.finishDownload(modelId, err)
				return
			}
			written += int64(n)
			s.mu.Lock()
			job.downloaded = written
			s.mu.Unlock()
		}
		if readErr != nil {
			if readErr != io.EOF {
				s.finishDownload(modelId, readErr)
				return
			}
			break
		}
	}

	if err := cache.Close(); err != nil {
		s.finishDownload(modelId, err)
		return
	}
	if err := renameWithRetry(target+".part", target); err != nil {
		s.finishDownload(modelId, err)
		return
	}
	s.finishDownload(modelId, nil)
}

// startDownload marks the transfer of a model host as running.
//
// The function returns false when a transfer of the same model already runs.
func (s *ModelStore) startDownload(modelId string) bool {
	job := s.job(modelId)
	s.mu.Lock()
	defer s.mu.Unlock()
	if job.running {
		return false
	}
	job.running = true
	job.err = nil
	job.downloaded = 0
	job.started = time.Now()
	return true
}

// finishDownload ends the transfer of a model host and records its failure.
func (s *ModelStore) finishDownload(modelId string, failure error) {
	job := s.job(modelId)
	s.mu.Lock()
	defer s.mu.Unlock()
	job.running = false
	job.err = failure
}

// serveModelFile serves a cached model file with the support of range requests.
func serveModelFile(c *gin.Context, path string, size int64) {
	file, err := openFileWithRetry(path)
	if err != nil && os.IsNotExist(err) && strings.HasSuffix(path, ".part") {
		finalPath := strings.TrimSuffix(path, ".part")
		if info, statErr := os.Stat(finalPath); statErr == nil && info.Size() > 0 {
			path = finalPath
			size = info.Size()
			file, err = openFileWithRetry(path)
		}
	}
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"success": false, "message": err.Error()})
		return
	}
	defer file.Close()

	c.Writer.Header().Set("Content-Type", "application/octet-stream")
	c.Writer.Header().Set("Accept-Ranges", "bytes")

	rangeHeader := c.GetHeader("Range")
	if rangeHeader == "" {
		c.Writer.Header().Set("Content-Length", fmt.Sprintf("%d", size))
		c.Writer.WriteHeader(http.StatusOK)
		_, _ = io.Copy(c.Writer, file)
		return
	}

	start, end, ok := parseRange(rangeHeader, size)
	if !ok {
		c.Writer.Header().Set("Content-Range", fmt.Sprintf("bytes */%d", size))
		c.Writer.WriteHeader(http.StatusRequestedRangeNotSatisfiable)
		return
	}
	if _, err := file.Seek(start, io.SeekStart); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"success": false, "message": err.Error()})
		return
	}
	c.Writer.Header().Set("Content-Range", fmt.Sprintf("bytes %d-%d/%d", start, end, size))
	c.Writer.Header().Set("Content-Length", fmt.Sprintf("%d", end-start+1))
	c.Writer.WriteHeader(http.StatusPartialContent)
	_, _ = io.CopyN(c.Writer, file, end-start+1)
}

// parseRange reads a byte range header.
func parseRange(header string, size int64) (int64, int64, bool) {
	if !strings.HasPrefix(header, "bytes=") {
		return 0, 0, false
	}
	parts := strings.SplitN(strings.TrimPrefix(header, "bytes="), "-", 2)
	if len(parts) != 2 {
		return 0, 0, false
	}
	start, err := parseInt64(parts[0])
	if err != nil {
		return 0, 0, false
	}
	end := size - 1
	if parts[1] != "" {
		parsed, err := parseInt64(parts[1])
		if err != nil {
			return 0, 0, false
		}
		end = parsed
	}
	if start < 0 || start >= size || end < start {
		return 0, 0, false
	}
	if end >= size {
		end = size - 1
	}
	return start, end, true
}

// parseInt64 parses a decimal number.
func parseInt64(value string) (int64, error) {
	var result int64
	if value == "" {
		return 0, fmt.Errorf("empty number")
	}
	for _, char := range value {
		if char < '0' || char > '9' {
			return 0, fmt.Errorf("invalid number %q", value)
		}
		result = result*10 + int64(char-'0')
	}
	return result, nil
}

// upstreamClient fetches the model files, it follows the redirects of the model hosts.
var upstreamClient = &http.Client{
	Timeout: 0,
	CheckRedirect: func(req *http.Request, via []*http.Request) error {
		if len(via) >= 10 {
			return fmt.Errorf("too many redirects")
		}
		return nil
	},
}

// renameWithRetry renames a file, retrying on transient Windows file lock errors.
func renameWithRetry(src, dst string) error {
	var err error
	for attempt := 0; attempt < 20; attempt++ {
		err = os.Rename(src, dst)
		if err == nil {
			return nil
		}
		time.Sleep(50 * time.Millisecond)
	}
	return err
}

// openFileWithRetry opens a file for reading, retrying on transient Windows file lock errors.
func openFileWithRetry(path string) (*os.File, error) {
	var file *os.File
	var err error
	for attempt := 0; attempt < 20; attempt++ {
		file, err = os.Open(path)
		if err == nil {
			return file, nil
		}
		time.Sleep(30 * time.Millisecond)
	}
	return nil, err
}
