// vimv2 is vi-mv-2, not vim-v2.
package main

import (
	"bufio"
	"fmt"
	"os"
	"os/exec"
	"runtime"

	"github.com/alecthomas/kong"
)

// TODO: prompt users on whether they want to retry (with the tmpfile reset or
// the same) if they enter invalid stuff.

var cli struct {
	Directory string `arg:"" default:"." type:"existingdir" help:"The directory in which you want to rename files."`
}

func main() {
	kong.Parse(&cli)

	// default to exit code 0, and defer an explicit exit with it
	exitCode := 0
	defer func() { os.Exit(exitCode) }()

	die := func(format string, a ...any) {
		fmt.Fprintf(os.Stderr, "%s: ", os.Args[0])
		fmt.Fprintf(os.Stderr, format, a...)
		fmt.Fprintln(os.Stderr)
		exitCode = 1
		// we use this instead of os.Exit so that we can run all cleanup that's
		// been deferred up 'till this point
		runtime.Goexit()
	}
	dieWrap := func(err error, format string, a ...any) {
		if err == nil {
			return
		}

		die(fmt.Sprintf("%s: %%s", format), append(a, err.Error())...)
	}

	// creating tmpfile and defering cleanup

	tmpfile, err := os.CreateTemp("", "vimv2")
	dieWrap(err, "creating tmpfile failed")

	defer func() {
		dieWrap(tmpfile.Close(), "closing tmpfile failed")
		dieWrap(os.Remove(tmpfile.Name()), "removing tmpfile failed")
	}()

	// detecting editor

	editor, editorFound := os.LookupEnv("EDITOR")
	if !editorFound {
		editor, editorFound = os.LookupEnv("VISUAL")
	}
	if !editorFound {
		die("no viable editor found, please set $EDITOR or $VISUAL")
	}

	// reading sources and writing to tmpfile

	entries, err := os.ReadDir(cli.Directory)
	dieWrap(err, "reading directory failed")

	srcs := make([]string, len(entries))
	for i, entry := range entries {
		srcs[i] = entry.Name()
		_, err := tmpfile.Write([]byte(entry.Name()))
		dieWrap(err, "writing to tmpfile failed")
		_, err = tmpfile.Write([]byte{byte('\n')})
		dieWrap(err, "writing to tmpfile failed")
	}

	// running editor

	cmd := exec.Command(editor, tmpfile.Name())
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	dieWrap(cmd.Run(), "running editor command failed")

	// reading the result of the edit and destination collision detection and
	// data structure setup. collisions detected here cannot be resolved
	// (without overwriting files) and will have to be revised before
	// continuing

	tmpfile.Seek(0, 0)
	scanner := bufio.NewScanner(tmpfile)

	dstSet := map[string]struct{}{}
	srcToDst := map[string]string{}

	for scanner.Scan() {
		if len(dstSet) > len(srcs) {
			die("tmpfile contains too many lines")
		}

		dst := scanner.Text()
		_, found := dstSet[dst]
		if found {
			die("duplicate destination \"%s\"", dst)
		}
		srcToDst[srcs[len(dstSet)]] = dst
		dstSet[dst] = struct{}{}
	}
	dieWrap(scanner.Err(), "reading tmpfile failed")

	if len(dstSet) < len(srcs) {
		die("tmpfile contains too few lines")
	}

	// movement

	dieWrap(moveAll(srcToDst, os.Rename, tmpClosure(srcToDst, dstSet)),
		"renaming failed")
}
