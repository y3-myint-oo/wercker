package main

import (
	. "gopkg.in/check.v1"

	"testing"

	"github.com/docker/docker/utils"
)

// Hook up gocheck into the "go test" runner.
func Test(t *testing.T) { TestingT(t) }

type MySuite struct{}

var _ = Suite(&MySuite{})

func (m *MySuite) TestPullParallelDownloads(c *C) {
	c.Skip("Race condition")
	testSteps := []struct {
		in       *utils.JSONMessage
		expected string
	}{
		{
			&utils.JSONMessage{
				ID:     "ubuntu:latest",
				Status: "The image you are pulling has been verified",
			},
			"The image you are pulling has been verified: ubuntu:latest\n",
		},
		{
			&utils.JSONMessage{
				ID:       "511136ea3c5a",
				Status:   "Pulling fs layer",
				Progress: &utils.JSONProgress{Current: 0, Start: 0, Total: 0},
			},
			"Pulling fs layer: 511136ea3c5a\n",
		},
		{
			&utils.JSONMessage{
				ID:       "c7b7c6419568",
				Status:   "Pulling fs layer",
				Progress: &utils.JSONProgress{Current: 0, Start: 0, Total: 0},
			},
			"Pulling fs layer: c7b7c6419568\n",
		},
		{
			&utils.JSONMessage{
				ID:       "511136ea3c5a",
				Status:   "Downloading",
				Progress: &utils.JSONProgress{Current: 0, Start: 0, Total: 100},
			},
			"Downloading: 511136ea3c5a (0%)",
		},
		{
			&utils.JSONMessage{
				ID:       "511136ea3c5a",
				Status:   "Downloading",
				Progress: &utils.JSONProgress{Current: 50, Start: 0, Total: 100},
			},
			"\rDownloading: 511136ea3c5a (50%)",
		},
		{
			&utils.JSONMessage{
				ID:       "c7b7c6419568",
				Status:   "Downloading",
				Progress: &utils.JSONProgress{Current: 0, Start: 0, Total: 100},
			},
			"\rDownloading: 511136ea3c5a (50%), Downloading: c7b7c6419568 (0%)",
		},
		{
			&utils.JSONMessage{
				ID:       "511136ea3c5a",
				Status:   "Download complete",
				Progress: &utils.JSONProgress{Current: 0, Start: 0, Total: 0},
			},
			"\rDownload complete: 511136ea3c5a                                \nDownloading: c7b7c6419568 (0%)",
		},
		{
			&utils.JSONMessage{
				ID:       "c7b7c6419568",
				Status:   "Downloading",
				Progress: &utils.JSONProgress{Current: 50, Start: 0, Total: 100},
			},
			"\rDownloading: c7b7c6419568 (50%)",
		},
		{
			&utils.JSONMessage{
				ID:       "c7b7c6419568",
				Status:   "Download complete",
				Progress: &utils.JSONProgress{Current: 0, Start: 0, Total: 0},
			},
			"\rDownload complete: c7b7c6419568\n",
		},
		{
			&utils.JSONMessage{
				ID:       "511136ea3c5a",
				Status:   "Extracting",
				Progress: &utils.JSONProgress{Current: 10, Start: 0, Total: 100},
			},
			"Extracting: 511136ea3c5a (10%)",
		},
		{
			&utils.JSONMessage{
				ID:       "511136ea3c5a",
				Status:   "Pull complete",
				Progress: &utils.JSONProgress{Current: 0, Start: 0, Total: 0},
			},
			"\rPull complete: 511136ea3c5a   \n",
		},
		{
			&utils.JSONMessage{
				ID:       "c7b7c6419568",
				Status:   "Extracting",
				Progress: &utils.JSONProgress{Current: 55, Start: 0, Total: 100},
			},
			"Extracting: c7b7c6419568 (55%)",
		},
		{
			&utils.JSONMessage{
				ID:       "c7b7c6419568",
				Status:   "Pull complete",
				Progress: &utils.JSONProgress{Current: 0, Start: 0, Total: 0},
			},
			"\rPull complete: c7b7c6419568   \n",
		},
		{
			&utils.JSONMessage{
				Status: "Status: Downloaded newer image for ubuntu:latest;",
			},
			"Status: Downloaded newer image for ubuntu:latest;\n",
		},
	}

	s := NewJSONMessageProcessor()
	for _, step := range testSteps {
		actual := s.ProcessJSONMessage(step.in)
		c.Assert(actual, Equals, step.expected)
	}
}

func (m *MySuite) TestPushParallelUploads(c *C) {
	testSteps := []struct {
		in       *utils.JSONMessage
		expected string
	}{
		{
			&utils.JSONMessage{
				Status: "The push refers to a repository [127.0.0.1:3000/bvdberg/pass] (len: 1)",
			},
			"Pushing to registry\n",
		},
		{
			&utils.JSONMessage{
				Status: "Sending image list",
			},
			"Sending image list\n",
		},
		{
			&utils.JSONMessage{
				Status: "Pushing repository 127.0.0.1:3000/bvdberg/pass (1 tags)",
			},
			"Pushing 1 tag(s)\n", // TODO
		},
		{
			&utils.JSONMessage{
				ID:       "511136ea3c5a",
				Status:   "Pushing",
				Progress: &utils.JSONProgress{Current: 0, Start: 0, Total: 0},
			},
			"Pushing: 511136ea3c5a",
		},
		{
			&utils.JSONMessage{
				ID:       "511136ea3c5a",
				Status:   "Buffering to disk",
				Progress: &utils.JSONProgress{Current: 10, Start: 0, Total: 0},
			},
			"\rBuffering to disk: 511136ea3c5a (10 B)",
		},
		// buffering done?
		{
			&utils.JSONMessage{
				ID:       "511136ea3c5a",
				Status:   "Pushing",
				Progress: &utils.JSONProgress{Current: 10, Start: 0, Total: 100},
			},
			"\rPushing: 511136ea3c5a (10%)           ",
		},
		{
			&utils.JSONMessage{
				ID:       "511136ea3c5a",
				Status:   "Image successfully pushed",
				Progress: &utils.JSONProgress{Current: 0, Start: 0, Total: 0},
			},
			"\rImage successfully pushed: 511136ea3c5a\n",
		},
		{
			&utils.JSONMessage{
				ID:       "c7b7c6419568",
				Status:   "Pushing",
				Progress: &utils.JSONProgress{Current: 0, Start: 0, Total: 0},
			},
			"Pushing: c7b7c6419568",
		},
		{
			&utils.JSONMessage{
				ID:       "c7b7c6419568",
				Status:   "Buffering to disk",
				Progress: &utils.JSONProgress{Current: 524287, Start: 0, Total: 0},
			},
			"\rBuffering to disk: c7b7c6419568 (511.9 KB)",
		},
		// Buffering done?
		{
			&utils.JSONMessage{
				ID:       "c7b7c6419568",
				Status:   "Pushing",
				Progress: &utils.JSONProgress{Current: 44, Start: 0, Total: 100},
			},
			"\rPushing: c7b7c6419568 (44%)               ",
		},
		{
			&utils.JSONMessage{
				ID:       "c7b7c6419568",
				Status:   "Image successfully pushed",
				Progress: &utils.JSONProgress{Current: 0, Start: 0, Total: 0},
			},
			"\rImage successfully pushed: c7b7c6419568\n",
		},
		{
			&utils.JSONMessage{
				Status: "Pushing tag for rev [a636b9702b50] on {http://127.0.0.1:3000/v1/repositories/bvdberg/pass/tags/build-549305dd56000d6d0700027e};",
			},
			"Pushing tag for image: a636b9702b50\n", // TODO
		},
	}

	s := NewJSONMessageProcessor()
	for _, step := range testSteps {
		actual := s.ProcessJSONMessage(step.in)
		c.Assert(actual, Equals, step.expected)
	}
}

func (m *MySuite) TestFormatDiskUnitBytes(c *C) {
	testSteps := []struct {
		in       int64
		expected string
	}{
		{1, "1 B"},
		{1023, "1023 B"},
		{1024, "1 KB"},
		{1025, "1 KB"},
		{1536, "1.5 KB"},
		{1048575, "1023.9 KB"},
		{1048576, "1 MB"},
		{1048577, "1 MB"},
		{1073741823, "1023.9 MB"},
		{1073741824, "1 GB"},
		{1073741825, "1 GB"},
		{2147483647, "1.9 GB"},
		{1099511628800, "1024 GB"},
		{1099511628801, "1024 GB"},
	}
	for _, step := range testSteps {
		actual := formatDiskUnit(step.in)
		c.Assert(actual, Equals, step.expected)
	}
}
