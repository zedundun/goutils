package net

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestGetNs(t *testing.T) {
	_, err := GetLocalNS()
	assert.Nil(t, err)
//	assert.NotEqual(t, len(ns.NicNS), 0)
}
