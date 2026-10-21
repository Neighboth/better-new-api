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
// time. A range request resumes an interrupted transfer.
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

	// Stream the download of the model host to the worker and cache it.
	upstream, err := http.NewRequest(http.MethodGet, modelDef.DownloadUrl, nil)
	if err != nil {
		c.JSON(http.StatusBadGateway, gin.H{"success": false, "message": err.Error()})
		return
	}
	upstream.Header.Set("User-Agent", "BYOG-Worker/1.0")

	// A worker that was interrupted resumes the transfer.
	resumeFrom := int64(0)
	if rangeHeader := c.GetHeader("Range"); rangeHeader != "" {
		if start, _, ok := parseRange(rangeHeader, modelDef.FileSize); ok && start > 0 {
			resumeFrom = start
			upstream.Header.Set("Range", fmt.Sprintf("bytes=%d-", start))
		}
	}

	resp, err := upstreamClient.Do(upstream)
	if err != nil {
		c.JSON(http.StatusBadGateway, gin.H{"success": false, "message": err.Error()})
		return
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusPartialContent {
		c.JSON(http.StatusBadGateway, gin.H{"success": false, "message": fmt.Sprintf("model host returned %d", resp.StatusCode)})
		return
	}

	job := s.job(modelId)
	job.total = resp.ContentLength
	if resumeFrom > 0 {
		job.total += resumeFrom
	}

	cache, err := os.OpenFile(target+".part", os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o644)
	if err != nil {
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
				job.err = err
				return
			}
			if _, err := cache.Write(buffer[:n]); err != nil {
				job.err = err
				return
			}
			written += int64(n)
			s.mu.Lock()
			job.downloaded = written + resumeFrom
			s.mu.Unlock()
		}
		if readErr != nil {
			if readErr != io.EOF {
				job.err = readErr
				return
			}
			break
		}
	}

	if err := cache.Close(); err != nil {
		job.err = err
		return
	}
	if resumeFrom == 0 {
		if err := os.Rename(target+".part", target); err != nil {
			job.err = err
			return
		}
	}
	s.mu.Lock()
	job.downloaded = written + resumeFrom
	job.err = nil
	s.mu.Unlock()
}

// serveModelFile serves a cached model file with the support of range requests.
func serveModelFile(c *gin.Context, path string, size int64) {
	file, err := os.Open(path)
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
