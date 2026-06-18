package tiktoken

import (
	"encoding/json"
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type parityCase struct {
	Name              string   `json:"name"`
	Encoding          string   `json:"encoding"`
	Text              string   `json:"text"`
	AllowedSpecial    []string `json:"allowed_special"`
	DisallowedSpecial []string `json:"disallowed_special"`
	Tokens            []int    `json:"tokens"`
}

func TestParityCases(t *testing.T) {
	data, err := os.ReadFile("testdata/parity_cases.json")
	require.NoError(t, err)

	var cases []parityCase
	require.NoError(t, json.Unmarshal(data, &cases))
	require.NotEmpty(t, cases)

	for _, tc := range cases {
		t.Run(tc.Name, func(t *testing.T) {
			enc, err := GetEncoding(tc.Encoding)
			require.NoError(t, err)

			tokens := enc.Encode(tc.Text, tc.AllowedSpecial, tc.DisallowedSpecial)
			assert.Equal(t, tc.Tokens, tokens)
			assert.Equal(t, tc.Text, enc.Decode(tokens))
		})
	}
}
