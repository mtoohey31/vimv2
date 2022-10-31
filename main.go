// vimv2 is vi-mv-2, not vim-v2.
package main

import (
	"fmt"
	"io"
	"os"
	"os/exec"
	"runtime"
	"strings"

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

	sources := make([]string, len(entries))

	for i, entry := range entries {
		sources[i] = entry.Name()
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

	// reading the result of the edit

	tmpfile.Seek(0, 0)
	tmpfileContents, err := io.ReadAll(tmpfile)
	dieWrap(err, "reading tmpfile failed")

	// validating the edit

	lines := strings.Split(string(tmpfileContents), "\n")
	lines = lines[:len(lines)-1]
	if len(lines) > len(sources) {
		die("tmpfile contains too many lines")
	} else if len(lines) < len(sources) {
		die("tmpfile contains too few lines")
	}

	// destination collision detection and data structure setup. collisions
	// detected here cannot be resolved (without overwriting files) and will
	// have to be revised before continuing

	newNames := make([]string, len(entries))
	sourceCollisionLookup := make(map[string]int)
	destCollisionLookup := make(map[string]struct{})

	for i, line := range lines {
		if _, found := destCollisionLookup[line]; found {
			die("duplicate destination \"%s\"", line)
		}
		newNames[i] = line
		sourceCollisionLookup[sources[i]] = i
		destCollisionLookup[line] = struct{}{}
	}

	// movement and temporary collision detection. collisions detected here can
	// be resolved by temporarily moving around files

	for i := 0; i < len(lines); i++ {
		source := sources[i]
		dest := newNames[i]

		// we don't need to check for collisions in the destination because we
		// already checked them in the previous loop
		destCollisionIndex, destCollisionExists := sourceCollisionLookup[dest]

		// remove the source from the collision lookup because it'll be moved
		// soon
		delete(sourceCollisionLookup, source)

		if destCollisionExists {
			// if the index of the collision item is the same as the current
			// index, the destination is the same as the source so we can skip
			// everything
			if destCollisionIndex == i {
				continue
			}

			tmpName := sources[destCollisionIndex] + ".tmp"
			// while the temporary name for the current file that's in the way
			// is still conflicting with existing files, we keep adding more
			// ".tmp"s to the end
			for {
				_, sourceCollision := sourceCollisionLookup[tmpName]
				// we have to check the destination on these iterations
				_, destCollision := destCollisionLookup[tmpName]
				if !(sourceCollision || destCollision) {
					break
				}
				tmpName += ".tmp"
			}
			dieWrap(os.Rename(sources[destCollisionIndex], tmpName), "rename failed")

			// reorganize the data structures related to the movement of the
			// tmpfile
			delete(sourceCollisionLookup, sources[destCollisionIndex])
			sources[destCollisionIndex] = tmpName
			sourceCollisionLookup[tmpName] = destCollisionIndex
		}

		dieWrap(os.Rename(source, dest), "rename failed")
	}
}
