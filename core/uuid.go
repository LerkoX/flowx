package core

import (
	"strings"

	"github.com/google/uuid"
)

// NewUUID 生成新的UUID（去除连字符格式）
func NewUUID() string {
	return strings.ReplaceAll(uuid.New().String(), "-", "")
}


