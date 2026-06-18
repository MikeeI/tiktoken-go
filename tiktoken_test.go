package tiktoken

import (
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"sync"
	"sync/atomic"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestSpecialTokenPolicy(t *testing.T) {
	ass := assert.New(t)
	req := require.New(t)
	enc, err := EncodingForModel("gpt-3.5-turbo-16k")
	req.NoError(err, "Encoding init should not fail")

	ass.NotPanics(func() {
		enc.Encode("hello "+ENDOFTEXT, []string{ENDOFTEXT}, nil)
	})
	ass.Panics(func() {
		enc.Encode("hello "+ENDOFTEXT, []string{ENDOFTEXT}, []string{ENDOFTEXT})
	})
	ass.Panics(func() {
		enc.Encode("hello "+ENDOFTEXT+ENDOFPROMPT, []string{ENDOFTEXT}, []string{allowedSpecialAll})
	})
}

func TestDecoding(t *testing.T) {
	ass := assert.New(t)
	req := require.New(t)
	enc, err := GetEncoding(MODEL_CL100K_BASE)
	req.NoError(err, "Encoding init should not fail")

	text := "hello world!你好，世界！"
	tokens := enc.Encode(text, nil, nil)
	ass.Equal(text, enc.Decode(tokens), "Decoding should be equal")
}

func TestLoadTiktokenBpe_InvalidLineReturnsError(t *testing.T) {
	path := t.TempDir() + "/bad.tiktoken"
	require.NoError(t, os.WriteFile(path, []byte("not-a-valid-line"), 0o600))

	_, err := loadTiktokenBpe(path)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "invalid tiktoken BPE line 1")
}

type urlRewriteLoader struct {
	realBase string
	fakeBase string
	inner    BpeLoader
}

func (u *urlRewriteLoader) LoadTiktokenBpe(url string) (map[string]int, error) {
	url = strings.Replace(url, u.realBase, u.fakeBase, 1)
	return u.inner.LoadTiktokenBpe(url)
}

func resetEncodingStateForTest(t *testing.T) {
	t.Helper()

	l.Lock()
	savedEncodingMap := encodingMap
	savedEncodingLoads := encodingLoads
	encodingMap = make(map[string]*encodingState)
	encodingLoads = make(map[string]*encodingLoad)
	l.Unlock()

	t.Cleanup(func() {
		l.Lock()
		encodingMap = savedEncodingMap
		encodingLoads = savedEncodingLoads
		l.Unlock()
	})
}

type staticBpeLoader struct {
	calls     atomic.Int64
	ranks     map[string]int
	started   chan struct{}
	release   chan struct{}
	startOnce sync.Once
}

func (s *staticBpeLoader) LoadTiktokenBpe(string) (map[string]int, error) {
	s.calls.Add(1)
	if s.started != nil {
		s.startOnce.Do(func() {
			close(s.started)
		})
	}
	if s.release != nil {
		<-s.release
	}
	if s.ranks != nil {
		return s.ranks, nil
	}
	return map[string]int{"a": 0}, nil
}

func TestGetEncoding_ErrorResponseNotCached(t *testing.T) {
	resetEncodingStateForTest(t)

	cacheDir := t.TempDir()

	ass := assert.New(t)
	req := require.New(t)
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "internal server error", http.StatusNotFound)
	}))
	t.Cleanup(func() {
		ts.Close()
		SetBpeLoader(NewDefaultBpeLoader())
	})

	loader := &urlRewriteLoader{
		realBase: BpeBaseURL,
		fakeBase: ts.URL,
		inner:    NewDefaultBpeLoader(),
	}
	SetBpeLoader(loader)

	got, err := GetEncoding(MODEL_O200K_BASE)
	ass.Nil(got)
	req.Error(err, "expected error when fetching encoding with bad response")

	entries, err := os.ReadDir(cacheDir)
	req.NoError(err)
	ass.Empty(entries, "expected empty cache dir after error")
}

func TestGetEncodingState_DeduplicatesConcurrentColdLoads(t *testing.T) {
	resetEncodingStateForTest(t)

	loader := &staticBpeLoader{
		ranks:   map[string]int{"a": 0},
		started: make(chan struct{}),
		release: make(chan struct{}),
	}
	SetBpeLoader(loader)
	t.Cleanup(func() {
		SetBpeLoader(NewDefaultBpeLoader())
	})

	const goroutineCount = 16
	start := make(chan struct{})
	errs := make(chan error, goroutineCount)
	states := make(chan *encodingState, goroutineCount)

	var ready sync.WaitGroup
	ready.Add(goroutineCount)
	var done sync.WaitGroup
	done.Add(goroutineCount)
	for range goroutineCount {
		go func() {
			defer done.Done()
			ready.Done()
			<-start

			state, err := getEncodingState(MODEL_CL100K_BASE)
			if err != nil {
				errs <- err
				return
			}
			states <- state
		}()
	}

	ready.Wait()
	close(start)
	<-loader.started
	close(loader.release)
	done.Wait()
	close(errs)
	close(states)

	for err := range errs {
		require.NoError(t, err)
	}

	var first *encodingState
	stateCount := 0
	for state := range states {
		require.NotNil(t, state)
		if first == nil {
			first = state
		} else {
			assert.Same(t, first, state)
		}
		stateCount++
	}
	assert.Equal(t, goroutineCount, stateCount)
	assert.Equal(t, int64(1), loader.calls.Load())
}

func TestGetEncoding_ReusesCoreBPE(t *testing.T) {
	resetEncodingStateForTest(t)

	loader := &staticBpeLoader{ranks: map[string]int{"a": 0}}
	SetBpeLoader(loader)
	t.Cleanup(func() {
		SetBpeLoader(NewDefaultBpeLoader())
	})

	first, err := GetEncoding(MODEL_CL100K_BASE)
	require.NoError(t, err)
	second, err := GetEncoding(MODEL_CL100K_BASE)
	require.NoError(t, err)

	assert.NotSame(t, first, second)
	assert.Same(t, first.bpe, second.bpe)
	assert.Equal(t, int64(1), loader.calls.Load())
}

func TestEncodingForModel_Names(t *testing.T) {
	for model := range MODEL_TO_ENCODING {
		// we don't support gpt2 model so far
		if model == MODEL_GPT2 {
			continue
		}
		t.Run("Check model "+model, func(t *testing.T) {
			testEncodingForModel(t, model)
		})
	}
}

func TestEncodingForModel_Prefixes(t *testing.T) {
	for prefix := range MODEL_PREFIX_TO_ENCODING {
		t.Run("Check prefix "+prefix, func(t *testing.T) {
			testEncodingForModel(t, prefix)
		})
	}
}

func testEncodingForModel(t *testing.T, model string) {
	t.Helper()

	text := "hello world"
	ass := assert.New(t)
	req := require.New(t)

	tkm, err := EncodingForModel(model)
	req.NoErrorf(err, "error getting encoding for model %q: %v", model, err)
	ass.NotNil(tkm, "Encoding for model %s should not be nil", model)

	encText := tkm.Encode(text, nil, nil)
	ass.Len(encText, 2, "Encoding len should be equal")

	decText := tkm.Decode(encText)
	ass.Equal(text, decText, "decoding mismatch - want: %s, got: %s", text, decText)
}
