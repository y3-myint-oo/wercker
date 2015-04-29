package main

import (
	"testing"

	"github.com/fsouza/go-dockerclient"
	"github.com/stretchr/testify/assert"
)

func boxByID(s string) (*Box, error) {
	return NewBox(&BoxConfig{ID: s}, emptyPipelineOptions(), &BoxOptions{})
}

func TestBoxName(t *testing.T) {
	_, err := boxByID("wercker/base@1.0.0")
	assert.NotNil(t, err)

	noTag, err := boxByID("wercker/base")
	assert.Nil(t, err)
	assert.Equal(t, "wercker/base:latest", noTag.Name)

	withTag, err := boxByID("wercker/base:foo")
	assert.Nil(t, err)
	assert.Equal(t, "wercker/base:foo", withTag.Name)
}

func TestBoxPortBindings(t *testing.T) {
	published := []string{
		"8000",
		"8001:8001",
		"127.0.0.1::8002",
		"127.0.0.1:8004:8003/udp",
	}
	checkBindings := [][]string{
		[]string{"8000/tcp", "", "8000"},
		[]string{"8001/tcp", "", "8001"},
		[]string{"8002/tcp", "127.0.0.1", "8002"},
		[]string{"8003/udp", "127.0.0.1", "8004"},
	}

	bindings := portBindings(published)
	assert.Equal(t, len(checkBindings), len(bindings))
	for _, check := range checkBindings {
		binding := bindings[docker.Port(check[0])]
		assert.Equal(t, check[1], binding[0].HostIP)
		assert.Equal(t, check[2], binding[0].HostPort)
	}
}
