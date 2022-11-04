package main

import (
	"bytes"
	"fmt"
	"io"
	"os"
	"sync"
	"testing"
)

func spawnCopyWorker(src io.Reader, dst io.Writer, wg *sync.WaitGroup, errCh chan error) {
	go func() {
		_, err1 := io.Copy(dst, src)
		if err1 != nil {
			errCh <- err1
		}

		// close the destination, if possible
		closer, ok := dst.(interface{ Close() error })
		if ok {
			err2 := closer.Close()

			// we only throw the first error down the channel, cause we're only
			// going to print one anyways
			if err1 == nil && err2 != nil {
				errCh <- err2
			}
		}

		wg.Done()
	}()
}

func redirectRecover(t *testing.T, f func(), stdin string) (stdout, stderr string) {
	// we're spawning three workers and each should only throw one error, so to
	// avoid blocking we only need to buffer three errors
	redirectErrCh := make(chan error, 3)
	var wg sync.WaitGroup
	wg.Add(3)

	// close the redirect channel once all workers are done, then we can wait
	// for the channel instead of the WaitGroup
	go func() {
		wg.Wait()
		close(redirectErrCh)
	}()

	// stdin
	stdinBuf := bytes.NewBuffer([]byte(stdin))
	stdinR, stdinW, err := os.Pipe()
	requireNoError(t, err)
	mockVar(t, &os.Stdin, stdinR)
	spawnCopyWorker(stdinBuf, stdinW, &wg, redirectErrCh)

	// stdout
	var stdoutBuf bytes.Buffer
	stdoutR, stdoutW, err := os.Pipe()
	requireNoError(t, err)
	mockVar(t, &os.Stdout, stdoutW)
	spawnCopyWorker(stdoutR, &stdoutBuf, &wg, redirectErrCh)

	var stderrBuf bytes.Buffer
	stderrR, stderrW, err := os.Pipe()
	requireNoError(t, err)
	mockVar(t, &os.Stderr, stderrW)
	spawnCopyWorker(stderrR, &stderrBuf, &wg, redirectErrCh)

	// when closed, indicates the run is complete
	runErrCh := make(chan error)

	// this has to happen on a separate goroutine in case f calls
	// runtime.Goexit, because that unwind doesn't get stopped by recover
	go func() {
		defer func() {
			defer close(runErrCh)
			if r := recover(); r != nil {
				runErrCh <- fmt.Errorf("f panicked with %v", r)
			}

			// try to close everything, then handle errors
			err1 := os.Stdout.Close()
			err2 := os.Stderr.Close()

			// pre-existing errors from redirectErrCh take priority, again, we
			// only show the first error
			err, found := <-redirectErrCh
			if found {
				runErrCh <- err
				return
			}
			if err1 != nil {
				runErrCh <- err1
				return
			}
			if err2 != nil {
				runErrCh <- err2
				return
			}
		}()

		f()
	}()

	err, found := <-runErrCh
	if found {
		t.Fatal(err)
	}

	return stdoutBuf.String(), stderrBuf.String()
}

func requireNoError(t *testing.T, err error, args ...any) {
	t.Helper()
	if err != nil {
		if len(args) > 0 {
			t.Fatalf(fmt.Sprintf("%s: %%s", args[0]),
				append(args[1:], err.Error())...)
		} else {
			t.Fatalf(err.Error())
		}
	}
}

func mockVar[T any](t *testing.T, variable *T, val T) {
	oldVal := *variable
	*variable = val
	t.Cleanup(func() {
		*variable = oldVal
	})
}
