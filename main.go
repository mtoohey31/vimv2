// vimv2 is vi-mv-2, not vim-v2.
package main

import (
	"bufio"
	"fmt"
	"io"
	"os"
	"os/exec"
	"runtime"

	"github.com/alecthomas/kong"
	"golang.org/x/term"
)

var cli struct {
	Directory string `arg:"" default:"." type:"existingdir" help:"The directory in which you want to rename files."`
}

// aliased to allow for test mocking
var exit = os.Exit

func main() {
	kong.Parse(&cli)

	// default to exit code 0, and defer an explicit exit with it
	exitCode := 0
	defer func() { exit(exitCode) }()

	warn := func(format string, a ...any) {
		fmt.Fprintf(os.Stderr, "%s: ", os.Args[0])
		fmt.Fprintf(os.Stderr, format, a...)
		fmt.Fprintln(os.Stderr)
	}
	die := func(format string, a ...any) {
		warn(format, a...)
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

	// detecting editor

	editor, editorFound := os.LookupEnv("EDITOR")
	if !editorFound {
		editor, editorFound = os.LookupEnv("VISUAL")
	}
	if !editorFound {
		die("no editor found, please set $EDITOR or $VISUAL")
	}

	// toctou is inevitable, we assume that nobody touches the files from the
	// time we read them until we exit

	// reading srcs

	entries, err := os.ReadDir(cli.Directory)
	dieWrap(err, "reading directory failed")

	srcs := make([]string, len(entries))
	for i, entry := range entries {
		srcs[i] = entry.Name()
	}

	// variable setup for the loop below
	tmpfile := (*os.File)(nil)
	tmpfileCreated, tmpfileClosed := false, false

	defer func() {
		// cleans up the last remaining tmpfile, if one exists
		if tmpfileCreated {
			if !tmpfileClosed {
				dieWrap(tmpfile.Close(), "closing tmpfile failed")
			}
			dieWrap(os.Remove(tmpfile.Name()), "removing tmpfile failed")
		}
	}()

	// maps for moving things
	var srcToDst map[string]string
	var dstSet map[string]struct{}

	// main input loop which continues until the user enters valid input or
	// exits intentionally

	for {
		// creating next tmpfile, if necessary

		if tmpfile == nil {
			tmpfile, err = os.CreateTemp("", "vimv2")
			dieWrap(err, "creating tmpfile failed")
			tmpfileCreated = true

			for _, src := range srcs {
				_, err := tmpfile.Write([]byte(src))
				dieWrap(err, "writing to tmpfile failed")
				_, err = tmpfile.Write([]byte{byte('\n')})
				dieWrap(err, "writing to tmpfile failed")
			}

			dieWrap(tmpfile.Close(), "closing tmpfile failed")
			tmpfileClosed = true
		}

		// running editor

		cmd := exec.Command(editor, tmpfile.Name())
		cmd.Stdin = os.Stdin
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr

		dieWrap(cmd.Run(), "running editor command failed")

		// reading the result of the edit, and validating as we go

		// intialize maps
		srcToDst = map[string]string{}
		dstSet = map[string]struct{}{}

		tmpfile, err = os.OpenFile(tmpfile.Name(), os.O_RDWR, 0)
		dieWrap(err, "reopening tmpfile failed")
		tmpfileClosed = false
		scanner := bufio.NewScanner(tmpfile)

		// indicates we exited the loop manually
		inputInvalid := false

		for scanner.Scan() {
			if len(dstSet) >= len(srcs) {
				warn("tmpfile contains too many lines")
				inputInvalid = true
				break
			}

			dst := scanner.Text()
			_, found := dstSet[dst]
			if found {
				warn("duplicate destination \"%s\"", dst)
				inputInvalid = true
				break
			}
			srcToDst[srcs[len(dstSet)]] = dst
			dstSet[dst] = struct{}{}
		}
		// if this is set, we don't need to check this because we already have
		// an error that we're going to warn about
		if !inputInvalid {
			dieWrap(scanner.Err(), "reading tmpfile failed")

			if len(dstSet) == len(srcs) {
				// everything's ok, so we can continue to moving
				break
			}

			// it can't contain too many because that would've caused an error
			// above, steting inputInvalid
			warn("tmpfile contains too few lines")
		}

	PROMPT:
		for {
			// print prompt
			fmt.Fprintf(os.Stderr, "[\033[1;31me\033[0mdit "+
				"existing/edit \033[1;31mn\033[0mew/\033[1;31mq\033[0muit]: ")

			// read 1 byte in raw mode so no enter is required
			var b [1]byte
			if term.IsTerminal(int(os.Stderr.Fd())) {
				oldState, err := term.MakeRaw(int(os.Stderr.Fd()))
				dieWrap(err, "failed to set terminal to raw mode")
				_, err = os.Stdin.Read(b[:])
				dieWrap(term.Restore(int(os.Stderr.Fd()), oldState),
					"failed to restore terminal state")
			} else {
				_, err = os.Stdin.Read(b[:])
				if err == io.EOF {
					fmt.Fprintln(os.Stderr)
					die("user exited")
				}
			}

			// print char (would be nice to just use terminal echo, but that's
			// not an option with x/term), and print newline so things show up
			// on the next line
			fmt.Fprintf(os.Stderr, "%c\n", b[0])

			// handle the read error
			dieWrap(err, "failed to read from stderr")

			// proceed according to user input
			switch b[0] {
			case 'n', 'N':
				dieWrap(tmpfile.Close(), "closing tmpfile failed")
				dieWrap(os.Remove(tmpfile.Name()), "removing tmpfile failed")
				tmpfile = nil
				fallthrough
			case 'e', 'E':
				break PROMPT
			case 3 /* ^C */, 4 /* ^D */, 'q', 'Q':
				die("user exited")
			default:
				warn("invalid selection '%c'", b[0])
			}
		}
	}

	// movement

	dieWrap(moveAll(srcToDst, os.Rename, tmpClosure(srcToDst, dstSet)),
		"renaming failed")
	runtime.Goexit()
}
