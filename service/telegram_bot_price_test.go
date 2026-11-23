package service

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestParseTelegramAdminIDs(t *testing.T) {
	assert.Equal(t, map[int64]bool{7944667086: true}, parseTelegramAdminIDs("7944667086"))
	assert.Equal(t, map[int64]bool{1: true, 2: true, 3: true}, parseTelegramAdminIDs(" 1, 2;3 "))
	assert.Empty(t, parseTelegramAdminIDs(""), "an empty allowlist keeps the bot closed")
	assert.Equal(t, map[int64]bool{5: true}, parseTelegramAdminIDs("5,abc,-1,0"))
}
