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

var exitCode = 0

func die(format string, a ...any) {
	fmt.Fprintf(os.Stderr, "%s: ", os.Args[0])
	fmt.Fprintf(os.Stderr, format, a...)
	exitCode = 1
	runtime.Goexit()
}

var cli struct {
	Directory string `arg:"" default:"." type:"existingdir" help:"The directory in which you want to rename files."`
}

func main() {
	kong.Parse(&cli)

	defer func() { os.Exit(exitCode) }()

	// creating tmpfile and defering cleanup

	tmpfile, err := os.CreateTemp("", "vimv2")
	if err != nil {
		die("creating tmpfile failed with %s\n", err.Error())
	}

	defer func() {
		err := tmpfile.Close()
		if err != nil {
			die("closing tmpfile failed with %s\n", err.Error())
		}
		err = os.Remove(tmpfile.Name())
		if err != nil {
			die("removing tmpfile failed with %s\n", err.Error())
		}
	}()

	// detecting editor

	editor := os.Getenv("EDITOR")
	if editor == "" {
		editor = os.Getenv("VISUAL")
	}
	if editor == "" {
		die("no viable editor found, please set $EDITOR or $VISUAL")
	}

	// reading sources and writing to tmpfile

	entries, err := os.ReadDir(cli.Directory)
	if err != nil {
		die("reading current directory failed with %s\n", err.Error())
	}

	sources := make([]string, len(entries))

	for i, entry := range entries {
		sources[i] = entry.Name()
		if _, err := tmpfile.Write([]byte(entry.Name())); err != nil {
			die("writing to tmpfile failed with %s\n", err.Error())
		}
		if _, err := tmpfile.Write([]byte{byte('\n')}); err != nil {
			die("writing to tmpfile failed with %s\n", err.Error())
		}
	}

	// running editor

	cmd := exec.Command(editor, tmpfile.Name())
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	if err := cmd.Run(); err != nil {
		die("running editor command failed with %s\n", err.Error())
	}

	// reading the result of the edit

	tmpfile.Seek(0, 0)
	tmpfileContents, err := io.ReadAll(tmpfile)
	if err != nil {
		die("reading tmpfile failed with %s\n", err.Error())
	}

	// validating the edit

	lines := strings.Split(string(tmpfileContents), "\n")
	lines = lines[:len(lines)-1]
	if len(lines) > len(sources) {
		die("tmpfile contains too many lines\n")
	} else if len(lines) < len(sources) {
		die("tmpfile contains too few lines\n")
	}

	// destination collision detection and data structure setup. collisions
	// detected here cannot be resolved (without overwriting files) and will
	// have to be revised before continuing

	newNames := make([]string, len(entries))
	sourceCollisionLookup := make(map[string]int)
	destCollisionLookup := make(map[string]struct{})

	for i, line := range lines {
		if _, found := destCollisionLookup[line]; found {
			die("duplicate destination \"%s\"\n", line)
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
			os.Rename(sources[destCollisionIndex], tmpName)

			// reorganize the data structures related to the movement of the
			// tmpfile
			delete(sourceCollisionLookup, sources[destCollisionIndex])
			sources[destCollisionIndex] = tmpName
			sourceCollisionLookup[tmpName] = destCollisionIndex
		}

		os.Rename(source, dest)
	}
}
