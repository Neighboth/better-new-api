package byog

import (
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	"github.com/QuantumNous/new-api/model"
	"github.com/glebarez/sqlite"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

// TestModelStoreServesTheModelFromTheHost makes sure that a worker receives the bytes
// of the model, that the file is cached and that a range request resumes a transfer.
func TestModelStoreServesTheModelFromTheHost(t *testing.T) {
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Length", "11")
		_, _ = w.Write([]byte("hello world"))
	}))
	defer upstream.Close()

	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	require.NoError(t, err)
	originalDB := model.DB
	model.DB = db
	t.Cleanup(func() { model.DB = originalDB })
	require.NoError(t, db.AutoMigrate(&model.ByogModel{}, &model.ByogSetting{}))

	require.NoError(t, db.Create(&model.ByogModel{
		ModelId:     "test-model",
		DownloadUrl: upstream.URL,
		FileSize:    11,
	}).Error)

	directory := t.TempDir()
	store := NewModelStore(directory)

	context, recorder := testContext()
	store.ServeModelFile(context, "test-model")
	assert.Equal(t, http.StatusOK, recorder.Code)
	assert.Equal(t, "hello world", recorder.Body.String())

	// The file is cached for the next worker.
	cached := filepath.Join(directory, "test-model.gguf")
	content, err := os.ReadFile(cached)
	require.NoError(t, err)
	assert.Equal(t, "hello world", string(content))

	// A range request returns the requested part of the file.
	partial, partialRecorder := testContext()
	partial.Request.Header.Set("Range", "bytes=6-10")
	store.ServeModelFile(partial, "test-model")
	assert.Equal(t, http.StatusPartialContent, partialRecorder.Code)
	assert.Equal(t, "world", partialRecorder.Body.String())
}
