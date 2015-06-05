package main

import (
	"bytes"
	"io/ioutil"
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestDockerFileCollectorSingle(t *testing.T) {
	client := dockerOrSkip(t)

	container, err := tempBusybox(client)
	assert.Nil(t, err)
	defer container.Remove()

	dfc := NewDockerFileCollector(client, container.ID)

	archive, errs := dfc.Collect("/etc/issue")
	var b bytes.Buffer

	select {
	case err := <-archive.SingleBytes("issue", &b):
		assert.Nil(t, err)
	case err := <-errs:
		assert.Nil(t, err)
		t.FailNow()
	}

	assert.Equal(t, "Welcome to Buildroot\n", b.String())
}

func TestDockerFileCollectorSingleNotFound(t *testing.T) {
	client := dockerOrSkip(t)

	container, err := tempBusybox(client)
	assert.Nil(t, err)
	defer container.Remove()

	dfc := NewDockerFileCollector(client, container.ID)

	// Fail first from docker client
	archive, errs := dfc.Collect("/notfound/file")
	var b bytes.Buffer
	select {
	case <-archive.SingleBytes("file", &b):
		t.FailNow()
	case err := <-errs:
		assert.Equal(t, err, ErrEmptyTarball)
	}

	// Or from archive
	archive, errs = dfc.Collect("/etc/issue")
	var b2 bytes.Buffer
	select {
	case err := <-archive.SingleBytes("notfound", &b2):
		assert.Equal(t, err, ErrEmptyTarball)
	case <-errs:
		t.FailNow()
	}

}

func TestDockerFileCollectorMulti(t *testing.T) {
	client := dockerOrSkip(t)

	container, err := tempBusybox(client)
	assert.Nil(t, err)
	defer container.Remove()

	dfc := NewDockerFileCollector(client, container.ID)

	archive, errs := dfc.Collect("/etc/network")
	var b bytes.Buffer

	select {
	case err := <-archive.SingleBytes("network/interfaces", &b):
		assert.Nil(t, err)
	case <-errs:
		t.FailNow()
	}

	check := `# Configure Loopback
auto lo
iface lo inet loopback

`
	assert.Equal(t, check, b.String())
}

func TestDockerFileCollectorMultiEmptyTarball(t *testing.T) {
	client := dockerOrSkip(t)

	container, err := tempBusybox(client)
	assert.Nil(t, err)
	defer container.Remove()

	dfc := NewDockerFileCollector(client, container.ID)

	archive, errs := dfc.Collect("/home/default")

	tmp, err := ioutil.TempDir("", "test-")
	assert.Nil(t, err)
	defer os.RemoveAll(tmp)

	select {
	case err := <-archive.Multi("default", tmp, 1024*1024*1000):
		assert.Equal(t, err, ErrEmptyTarball)
	case <-errs:
		t.FailNow()
	}
}

func TestDockerFileCollectorMultiNotFound(t *testing.T) {
	client := dockerOrSkip(t)

	container, err := tempBusybox(client)
	assert.Nil(t, err)
	defer container.Remove()

	dfc := NewDockerFileCollector(client, container.ID)

	archive, errs := dfc.Collect("/notfound")

	tmp, err := ioutil.TempDir("", "test-")
	assert.Nil(t, err)
	defer os.RemoveAll(tmp)

	select {
	case <-archive.Multi("default", tmp, 1024*1024*1000):
		t.FailNow()
	case err := <-errs:
		assert.Equal(t, err, ErrEmptyTarball)
	}
}
