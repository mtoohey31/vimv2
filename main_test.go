//go:generate go build -o testdata testdata/mockeditor.go

package main

import (
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"sort"
	"testing"
)

const prompt = "[\033[1;31me\033[0mdit existing/edit " +
	"\033[1;31mn\033[0mew/\033[1;31mq\033[0muit]: "

func Test_main(t *testing.T) {
	// assumes the tests are run from the root of the repository
	cwd, err := os.Getwd()
	requireNoError(t, err)
	// restore cwd, since subtests will change it
	t.Cleanup(func() { requireNoError(t, os.Chdir(cwd)) })

	mockEditorPath := filepath.Join(cwd, "testdata", "mockeditor")
	if runtime.GOOS == "windows" {
		mockEditorPath += ".exe"
	}

	t.Log(mockEditorPath)

	// generate the mock editor if it doesn't already exist
	_, err = os.Stat(mockEditorPath)
	if err != nil && errors.Is(err, os.ErrNotExist) {
		requireNoError(t, exec.Command("go", "generate").Run())
	} else {
		requireNoError(t, err)
	}

	nonExecutableEditorPath := filepath.Join(t.TempDir(), "nonexecutable")
	requireNoError(t, os.WriteFile(nonExecutableEditorPath, nil, 0o644))

	tests := []struct {
		description string

		preTest      func(t *testing.T)
		stdin        string
		createdFiles []string

		expectedFiles                  []string
		expectedStdout, expectedStderr string
		expectedExitCode               int
	}{
		{
			description: "happy path simple",
			preTest: func(t *testing.T) {
				t.Setenv("EDITOR", mockEditorPath)
				countFile := filepath.Join(t.TempDir(), "count")
				requireNoError(t, os.WriteFile(countFile, []byte{'0'}, 0o644))
				t.Setenv("MOCK_EDITOR_COUNT_FILE", countFile)
				t.Setenv("MOCK_EDITOR_OUTPUT_0", `d file
e file
f file
`)
				t.Setenv("MOCK_EDITOR_EXIT_CODE_0", "0")
			},
			createdFiles: []string{
				"a file",
				"b file",
				"c file",
			},
			expectedFiles: []string{
				"d file",
				"e file",
				"f file",
			},
			expectedStdout: "mock editor run 0\n",
			expectedStderr: "mock editor run 0\n",
		},

		{
			description:      "no editor",
			expectedStderr:   "self: no editor found, please set $EDITOR or $VISUAL\n",
			expectedExitCode: 1,
		},
		{
			description: "editor not executable, $VISUAL respected",
			preTest: func(t *testing.T) {
				t.Setenv("VISUAL", nonExecutableEditorPath)
			},
			expectedStderr: fmt.Sprintf("self: running editor command failed: "+
				"fork/exec %s: permission denied\n", nonExecutableEditorPath),
			expectedExitCode: 1,
		},
		{
			description: "editor exits with non-zero",
			preTest: func(t *testing.T) {
				t.Setenv("EDITOR", mockEditorPath)
				countFile := filepath.Join(t.TempDir(), "count")
				requireNoError(t, os.WriteFile(countFile, []byte{'0'}, 0o644))
				t.Setenv("MOCK_EDITOR_COUNT_FILE", countFile)
				t.Setenv("MOCK_EDITOR_OUTPUT_0", "")
				t.Setenv("MOCK_EDITOR_EXIT_CODE_0", "15")
			},
			expectedStdout: "mock editor run 0\n",
			expectedStderr: `mock editor run 0
self: running editor command failed: exit status 15
`,
			expectedExitCode: 1,
		},
		{
			description: "too many lines, invalid selection, prompt EOF",
			preTest: func(t *testing.T) {
				t.Setenv("EDITOR", mockEditorPath)
				countFile := filepath.Join(t.TempDir(), "count")
				requireNoError(t, os.WriteFile(countFile, []byte{'0'}, 0o644))
				t.Setenv("MOCK_EDITOR_COUNT_FILE", countFile)
				t.Setenv("MOCK_EDITOR_OUTPUT_0", `d file
e file
f file
g file
`)
				t.Setenv("MOCK_EDITOR_EXIT_CODE_0", "0")
			},
			stdin: "?",
			createdFiles: []string{
				"a file",
				"b file",
				"c file",
			},
			expectedFiles: []string{
				"a file",
				"b file",
				"c file",
			},
			expectedStdout: "mock editor run 0\n",
			expectedStderr: `mock editor run 0
self: tmpfile contains too many lines
` + prompt + `?
self: invalid selection '?'
` + prompt + `
self: user exited
`,
			expectedExitCode: 1,
		},
		{
			description: "duplicate destination, retried with new, too few, retried with existing, empty, quit",
			preTest: func(t *testing.T) {
				t.Setenv("EDITOR", mockEditorPath)
				countFile := filepath.Join(t.TempDir(), "count")
				requireNoError(t, os.WriteFile(countFile, []byte{'0'}, 0o644))
				t.Setenv("MOCK_EDITOR_COUNT_FILE", countFile)
				t.Setenv("MOCK_EDITOR_OUTPUT_0", `d file
e file
e file
`)
				t.Setenv("MOCK_EDITOR_OUTPUT_1", `d file
e file
`)
				t.Setenv("MOCK_EDITOR_OUTPUT_2", "")
				t.Setenv("MOCK_EDITOR_EXIT_CODE_0", "0")
				t.Setenv("MOCK_EDITOR_EXIT_CODE_1", "0")
				t.Setenv("MOCK_EDITOR_EXIT_CODE_2", "0")
			},
			stdin: "nEq",
			createdFiles: []string{
				"a file",
				"b file",
				"c file",
			},
			expectedFiles: []string{
				"a file",
				"b file",
				"c file",
			},
			expectedStdout: `mock editor run 0
mock editor run 1
mock editor run 2
`,
			expectedStderr: `mock editor run 0
self: duplicate destination "e file"
` + prompt + `n
mock editor run 1
self: tmpfile contains too few lines
` + prompt + `E
mock editor run 2
self: tmpfile contains too few lines
` + prompt + `q
self: user exited
`,
			expectedExitCode: 1,
		},
	}

	// prevent external env from polluting tests
	for _, envVar := range [2]string{"EDITOR", "VISUAL"} {
		oldVal, found := os.LookupEnv(envVar)
		if found {
			requireNoError(t, os.Unsetenv(envVar))
			t.Cleanup(func() {
				requireNoError(t, os.Setenv(envVar, oldVal))
			})
		}
	}

	// clear args
	mockVar(t, &os.Args, []string{"self"})

	// exit mocking
	var actualExitCode int
	mockVar(t, &exit, func(exitCode int) {
		actualExitCode = exitCode
	})

	for _, test := range tests {
		t.Run(test.description, func(t *testing.T) {
			// setup

			tempDir := t.TempDir()

			// cwd will be restored by at the end of the test as a whole
			os.Chdir(tempDir)

			for _, file := range test.createdFiles {
				requireNoError(t, os.WriteFile(file, nil, 0o644))
			}

			// clear actualExitCode so we can tell when it didn't get set
			actualExitCode = -1

			if test.preTest != nil {
				test.preTest(t)
			}

			// SUT with stdin/stdout/stderr mocking
			actualStdout, actualStderr := redirectRecover(t, main, test.stdin)

			// a ssertions
			if test.expectedExitCode != actualExitCode {
				t.Errorf("expected exit code: %d did not match actual exit "+
					"code: %d", test.expectedExitCode, actualExitCode)
			}
			if test.expectedStdout != actualStdout {
				t.Errorf("expected stdout:\n%s\ndid not match actual "+
					"stdout:\n%s", test.expectedStdout, actualStdout)
			}
			if test.expectedStderr != actualStderr {
				t.Errorf("expected stderr:\n%s\ndid not match actual "+
					"stderr:\n%s", test.expectedStderr, actualStderr)
			}

			actualEntries, err := os.ReadDir(".")
			requireNoError(t, err)

			actualFiles := make([]string, len(actualEntries))
			for i, e := range actualEntries {
				actualFiles[i] = e.Name()
			}

			if len(test.expectedFiles) == len(actualFiles) {
				sort.Strings(test.expectedFiles)
				sort.Strings(actualFiles)
				for i, expectedFile := range test.expectedFiles {
					if expectedFile != actualFiles[i] {
						t.Errorf(
							"expected files: %v didn't match actual files: %v",
							test.expectedFiles, actualFiles)
					}
				}
			} else {
				t.Errorf("expected files: %v didn't match actual files: %v",
					test.expectedFiles, actualFiles)
			}
		})
	}
}
