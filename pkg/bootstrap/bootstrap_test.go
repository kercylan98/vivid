package bootstrap

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestNewActorSystem(t *testing.T) {
	system := NewActorSystem()
	assert.NotNil(t, system)
}
