//go:build ignore

package main

import (
	"fmt"
	"os"
	"strconv"
)

// this program can be controlled via the following environment variables:
//
// - $MOCK_EDITOR_COUNT_FILE: which contains a path to a file that will store
//   the number of times the editor has been invoked this test run
// - $MOCK_EDITOR_OUTPUT_n: the data to write to os.Args[1] for run n
// - $MOCK_EDITOR_EXIT_CODE_n: the code to exit with for run n

func main() {
	countFile, ok := os.LookupEnv("MOCK_EDITOR_COUNT_FILE")
	if !ok {
		panic("$MOCK_EDITOR_COUNT_FILE unset")
	}

	b, err := os.ReadFile(countFile)
	if err != nil {
		panic(err)
	}

	n, err := strconv.Atoi(string(b))
	if err != nil {
		panic(err)
	}

	fmt.Printf("mock editor run %d\n", n)
	fmt.Fprintf(os.Stderr, "mock editor run %d\n", n)

	err = os.WriteFile(countFile, []byte(strconv.Itoa(n+1)), 0o644)
	if err != nil {
		panic(err)
	}

	output, ok := os.LookupEnv(fmt.Sprintf("MOCK_EDITOR_OUTPUT_%d", n))
	if !ok {
		panic(fmt.Sprintf("$MOCK_EDITOR_OUTPUT_%d unset", n))
	}

	err = os.WriteFile(os.Args[1], []byte(output), 0o644)
	if err != nil {
		panic(err)
	}

	ecStr, ok := os.LookupEnv(fmt.Sprintf("MOCK_EDITOR_EXIT_CODE_%d", n))
	if !ok {
		panic(fmt.Sprintf("$MOCK_EDITOR_EXIT_CODE_%d unset", n))
	}

	ec, err := strconv.Atoi(ecStr)
	if err != nil {
		panic(err)
	}

	os.Exit(ec)
}
