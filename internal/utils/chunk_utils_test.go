package utils

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestJoinBlocksIntoChunks(t *testing.T) {
	tests := []struct {
		name   string
		blocks []string
		limit  int
		want   []string
	}{
		{"no blocks", nil, 10, nil},
		{"everything fits", []string{"ab", "cd", "ef"}, 10, []string{"abcdef"}},
		{"exact limit", []string{"abcde", "fghij"}, 10, []string{"abcdefghij"}},
		{"split between blocks", []string{"abcd", "efgh", "ijkl"}, 10, []string{"abcdefgh", "ijkl"}},
		{"oversized block stays whole", []string{"ab", strings.Repeat("x", 15), "cd"}, 10,
			[]string{"ab", strings.Repeat("x", 15), "cd"}},
		{"emoji count as two units", []string{"😀😀😀", "😀😀😀"}, 10, []string{"😀😀😀", "😀😀😀"}},
		{"empty blocks are kept inside chunks", []string{"", "ab", ""}, 10, []string{"ab"}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := JoinBlocksIntoChunks(tt.blocks, tt.limit)
			assert.Equal(t, tt.want, got)
			assert.Equal(t, strings.Join(tt.blocks, ""), strings.Join(got, ""), "no content is lost")
		})
	}
}
