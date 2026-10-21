package byog

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"sync"
	"testing"

	"github.com/QuantumNous/new-api/model"
	"github.com/glebarez/sqlite"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

// TestModelStoreSharesTheTransferOfTheModelHost makes sure that the workers that
// ask for a part of a model share a single transfer of the model host.
//
// Every worker that asked for a part would start its own transfer without the
// sharing, the model host would be loaded with the requests of the workers and
// the cache would hold a part of a model.
func TestModelStoreSharesTheTransferOfTheModelHost(t *testing.T) {
	content := []byte("0123456789abcdefghij")

	var mutex sync.Mutex
	transfers := 0
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		mutex.Lock()
		transfers++
		mutex.Unlock()
		w.Header().Set("Content-Length", fmt.Sprintf("%d", len(content)))
		_, _ = w.Write(content)
	}))
	defer upstream.Close()

	// The workers work at the same time, the database has to serve more than one
	// connection.
	db, err := gorm.Open(sqlite.Open(filepath.Join(t.TempDir(), "byog-test.db")), &gorm.Config{})
	require.NoError(t, err)
	originalDB := model.DB
	model.DB = db
	t.Cleanup(func() { model.DB = originalDB })
	sqlDB, err := db.DB()
	require.NoError(t, err)
	t.Cleanup(func() { _ = sqlDB.Close() })
	require.NoError(t, db.AutoMigrate(&model.ByogModel{}, &model.ByogSetting{}))

	require.NoError(t, db.Create(&model.ByogModel{
		ModelId:     "shared-model",
		DownloadUrl: upstream.URL,
		FileSize:    int64(len(content)),
	}).Error)

	store := NewModelStore(t.TempDir())

	// The workers of the installation ask for their ranges at the same time.
	var group sync.WaitGroup
	for index := 0; index < 4; index++ {
		group.Add(1)
		go func(start int) {
			defer group.Done()
			context, recorder := testContext()
			context.Request.Header.Set("Range", fmt.Sprintf("bytes=%d-%d", start, start+4))
			store.ServeModelFile(context, "shared-model")
			assert.Equal(t, http.StatusPartialContent, recorder.Code)
			assert.Equal(t, string(content[start:start+5]), recorder.Body.String())
		}(index * 5)
	}
	group.Wait()

	mutex.Lock()
	defer mutex.Unlock()
	assert.Equal(t, 1, transfers, "the model host must be asked only once")
}
