package tiktoken

import (
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
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

type urlRewriteLoader struct {
	realBase string
	fakeBase string
	inner    BpeLoader
}

func (u *urlRewriteLoader) LoadTiktokenBpe(url string) (map[string]int, error) {
	url = strings.Replace(url, u.realBase, u.fakeBase, 1)
	return u.inner.LoadTiktokenBpe(url)
}

func TestGetEncoding_ErrorResponseNotCached(t *testing.T) {
	cacheDir := t.TempDir()
	t.Setenv("TIKTOKEN_CACHE_DIR", cacheDir)

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
