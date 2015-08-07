package main

import (
	"fmt"
	"io"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"golang.org/x/net/context"
)

type FakeTransport struct {
	stdin      io.Reader
	stdout     io.Writer
	stderr     io.Writer
	cancelFunc context.CancelFunc

	inchan  chan string
	outchan chan string
}

func (t *FakeTransport) Attach(sessionCtx context.Context, stdin io.Reader, stdout, stderr io.Writer) (context.Context, error) {
	fakeContext, cancel := context.WithCancel(sessionCtx)
	t.cancelFunc = cancel
	t.stdin = stdin
	t.stdout = stdout
	t.stderr = stderr

	t.inchan = make(chan string)
	t.outchan = make(chan string)

	go func() {
		for {
			var p []byte
			p = make([]byte, 1024)
			i, err := t.stdin.Read(p)
			s := string(p[:i])
			rootLogger.Println(fmt.Sprintf("(test)  stdin: %q", s))
			t.inchan <- s
			if err != nil {
				close(t.inchan)
				return
			}
		}
	}()

	go func() {
		for {
			s := <-t.outchan
			rootLogger.Println(fmt.Sprintf("(test) stdout: %q", s))
			_, err := t.stdout.Write([]byte(s))
			if err != nil {
				close(t.outchan)
				return
			}
		}
	}()

	return fakeContext, nil
}

func (t *FakeTransport) Cancel() {
	t.cancelFunc()
}

func (t *FakeTransport) ListenAndRespond(exit int, recv []string) {
	for {
		s := <-t.inchan
		// If this is the last string send our stuff and echo the status code
		if strings.HasPrefix(s, "echo") && strings.HasSuffix(s, "$?\n") {
			parts := strings.Split(s, " ")
			for _, x := range recv {
				t.outchan <- x
			}
			t.outchan <- fmt.Sprintf("%s %d", parts[1], exit)
			return
		}
	}
}

func fakeSessionOptions() *PipelineOptions {
	return &PipelineOptions{
		GlobalOptions:     &GlobalOptions{Debug: true},
		NoResponseTimeout: 100,
		CommandTimeout:    100,
	}
}

func FakeSession(t *testing.T, opts *PipelineOptions) (context.Context, context.CancelFunc, *Session, *FakeTransport) {
	if opts == nil {
		opts = fakeSessionOptions()
	}
	transport := &FakeTransport{}
	topCtx, cancel := context.WithCancel(context.Background())
	topCtx = NewEmitterContext(topCtx)
	session := NewSession(opts, transport)

	sessionCtx, err := session.Attach(topCtx)
	assert.Nil(t, err)
	return sessionCtx, cancel, session, transport
}

func fakeSentinel(s string) func() string {
	return func() string {
		return s
	}
}

func TestSessionSend(t *testing.T) {
	setup(t)
	sessionCtx, _, session, transport := FakeSession(t, nil)

	go func() {
		session.Send(sessionCtx, false, "foo")
	}()

	s := <-transport.inchan
	assert.Equal(t, "foo\n", s)
}

func TestSessionSendCancelled(t *testing.T) {
	setup(t)
	sessionCtx, cancel, session, _ := FakeSession(t, nil)
	cancel()

	errchan := make(chan error)
	go func() {
		errchan <- session.Send(sessionCtx, false, "foo")
	}()

	assert.NotNil(t, <-errchan)
}

func TestSessionSendChecked(t *testing.T) {
	setup(t)
	sessionCtx, _, session, transport := FakeSession(t, nil)

	stepper := NewStepper()
	go func() {
		transport.ListenAndRespond(0, []string{"foo\n"})
		stepper.Wait()
		transport.ListenAndRespond(1, []string{"bar\n"})
	}()

	// Success
	exit, recv, err := session.SendChecked(sessionCtx, "foo")
	assert.Nil(t, err)
	assert.Equal(t, 0, exit)
	assert.Equal(t, "foo\n", recv[0])

	stepper.Step()
	// Non-zero Exit
	exit, recv, err = session.SendChecked(sessionCtx, "lala")
	assert.NotNil(t, err)
	assert.Equal(t, 1, exit)
	assert.Equal(t, "bar\n", recv[0])
}

func TestSessionSendCheckedCommandTimeout(t *testing.T) {
	setup(t)
	opts := fakeSessionOptions()
	opts.CommandTimeout = 0
	sessionCtx, _, session, transport := FakeSession(t, opts)

	go func() {
		transport.ListenAndRespond(0, []string{"foo\n"})
	}()

	exit, recv, err := session.SendChecked(sessionCtx, "foo")
	assert.NotNil(t, err)
	// We timed out so -1
	assert.Equal(t, -1, exit)
	// We sent some text so we should have gotten that at least
	assert.Equal(t, 1, len(recv))
}

func TestSessionSendCheckedNoResponseTimeout(t *testing.T) {
	setup(t)
	opts := fakeSessionOptions()
	opts.NoResponseTimeout = 0
	sessionCtx, _, session, transport := FakeSession(t, opts)

	go func() {
		// Just listen and never send anything
		for {
			<-transport.inchan
		}
	}()

	exit, recv, err := session.SendChecked(sessionCtx, "foo")
	assert.NotNil(t, err)
	assert.Equal(t, -1, exit)
	assert.Equal(t, 0, len(recv))
}

func TestSessionSendCheckedEarlyExit(t *testing.T) {
	setup(t)
	sessionCtx, _, session, transport := FakeSession(t, nil)

	stepper := NewStepper()
	randomSentinel = fakeSentinel("test-sentinel")

	go func() {
		for {
			stepper.Wait()
			<-transport.inchan
		}
	}()

	go func() {
		stepper.Step() // "foo"
		// Wait 5 milliseconds because Send has short delay
		stepper.Step(5) // "echo test-sentinel $?"
		transport.outchan <- "foo"
		transport.Cancel()
		transport.outchan <- "bar"
	}()

	exit, recv, err := session.SendChecked(sessionCtx, "foo")
	assert.NotNil(t, err)
	assert.Equal(t, -1, exit)
	assert.Equal(t, 2, len(recv), "should have gotten two lines of output")

}

func TestSessionSmartSplitLines(t *testing.T) {
	sentinel := "FOO9000"
	sentinelLine := "FOO9000 1\n"
	testLine := "some garbage\n"

	// Test easy normal return
	simpleLine := sentinelLine
	simpleLines := smartSplitLines(simpleLine, sentinel)
	assert.Equal(t, 1, len(simpleLines))
	assert.Equal(t, sentinelLine, simpleLines[0])
	simpleFound, simpleExit := checkLine(simpleLines[0], sentinel)
	assert.Equal(t, true, simpleFound)
	assert.Equal(t, 1, simpleExit)

	// Test return on same logical line as other stuff
	mixedLine := fmt.Sprintf("%s%s", testLine, sentinelLine)
	mixedLines := smartSplitLines(mixedLine, sentinel)
	assert.Equal(t, 2, len(mixedLines))
	assert.Equal(t, testLine, mixedLines[0])
	assert.Equal(t, sentinelLine, mixedLines[1])
	mixedFound, mixedExit := checkLine(mixedLines[1], sentinel)
	assert.Equal(t, true, mixedFound)
	assert.Equal(t, 1, mixedExit)

	// Test no return
	uselessLine := fmt.Sprintf("%s%s", testLine, testLine)
	uselessLines := smartSplitLines(uselessLine, sentinel)
	assert.Equal(t, 1, len(uselessLines))
	assert.Equal(t, uselessLine, uselessLines[0])
	uselessFound, _ := checkLine(uselessLines[0], sentinel)
	assert.Equal(t, false, uselessFound)
}
