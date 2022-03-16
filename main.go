package main

import (
	"fmt"
	"io"
	"io/ioutil"
	"os"
	"os/exec"
	"runtime"
	"strings"

	"mtoohey.com/which"
)

// TODO: prompt users on whether they want to retry (with the tmpfile reset or
// the same) if they enter invalid stuff.

var editors []string = []string{"nvim", "vim", "vi"}

func main() {
	res := 0
	defer func() { os.Exit(res) }()

	// creating tmpfile, finding path, and defering cleanup

	tmpfile, err := ioutil.TempFile("", "vimv2")

	defer func() {
		err := tmpfile.Close()
		if err != nil {
			fmt.Fprintf(os.Stderr, "closing tmpfile failed with %s\n", err.Error())
			res = 1
			runtime.Goexit()
		}
	}()

	tmpfilePath, err := os.Readlink(fmt.Sprintf("/proc/%d/fd/%d", os.Getpid(),
		tmpfile.Fd()))
	if err != nil {
		fmt.Fprintf(os.Stderr, "failed to find path of tmpfile with %s\n",
			err.Error())
		res = 1
		runtime.Goexit()
	}

	// NOTE: since defers are executed in LIFO order, this delete will occur
	// before the close, but that's fine, the os should handle it without error
	defer func() {
		err := os.Remove(tmpfilePath)
		if err != nil {
			fmt.Fprintf(os.Stderr, "deleting tmpfile failed with %s\n", err.Error())
			res = 1
			runtime.Goexit()
		}
	}()

	if err != nil {
		fmt.Fprintln(os.Stderr, "failed to create tmpfile")
		res = 1
		runtime.Goexit()
	}

	// detecting editor

	editor := os.Getenv("EDITOR")
	if editor == "" {
		editor = os.Getenv("VISUAL")
		if editor == "" {
			var err error = nil
			for i := 0; i < len(editors) && err != nil; i++ {
				editor, err = which.Which(editors[i])
			}
		}
	}
	if editor == "" {
		fmt.Fprintf(os.Stderr,
			"no viable editor found, please set $EDITOR, $VISUAL, or install one of %v",
			editors)
		res = 1
		runtime.Goexit()
	}

	// reading sources and writing to tmpfile

	entries, err := os.ReadDir(".")
	if err != nil {
		fmt.Fprintf(os.Stderr, "reading current directory failed with %s\n",
			err.Error())
		res = 1
		runtime.Goexit()
	}

	sources := make([]string, len(entries))

	for i, entry := range entries {
		sources[i] = entry.Name()
		if _, err := tmpfile.Write([]byte(entry.Name())); err != nil {
			fmt.Fprintf(os.Stderr, "writing to tmpfile failed with %s\n", err.Error())
			res = 1
			runtime.Goexit()
		}
		if _, err := tmpfile.Write([]byte{byte('\n')}); err != nil {
			fmt.Fprintf(os.Stderr, "writing to tmpfile failed with %s\n", err.Error())
			res = 1
			runtime.Goexit()
		}
	}

	// running editor

	cmd := exec.Command(editor, tmpfilePath)
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	if err := cmd.Run(); err != nil {
		fmt.Fprintf(os.Stderr, "running editor command failed with %s\n",
			err.Error())
		res = 1
		runtime.Goexit()
	}

	// reading the result of the edit

	tmpfile.Seek(0, 0)
	tmpfileContents, err := io.ReadAll(tmpfile)
	if err != nil {
		fmt.Fprintf(os.Stderr, "reading tmpfile failed with %s\n", err.Error())
		res = 1
		runtime.Goexit()
	}

	// validating the edit

	lines := strings.Split(string(tmpfileContents), "\n")
	lines = lines[:len(lines)-1]
	if len(lines) > len(sources) {
		fmt.Fprintln(os.Stderr, "tmpfile contains too many lines")
		res = 1
		runtime.Goexit()
	} else if len(lines) < len(sources) {
		fmt.Fprintln(os.Stderr, "tmpfile contains too few lines")
		res = 1
		runtime.Goexit()
	}

	// destination collision detection and data structure setup. collisions
	// detected here cannot be resolved (without overwriting files) and will have
	// to be revised before continuing

	newNames := make([]string, len(entries))
	sourceCollisionLookup := make(map[string]int)
	destCollisionLookup := make(map[string]struct{})

	for i, line := range lines {
		if _, found := destCollisionLookup[line]; found {
			fmt.Fprintf(os.Stderr, "duplicate destination \"%s\"\n", line)
			res = 1
			runtime.Goexit()
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

		// remove the source from the collision lookup because it'll be moved soon
		delete(sourceCollisionLookup, source)

		if destCollisionExists {
			// if the index of the collision item is the same as the current index,
			// the destination is the same as the source so we can skip everything
			if destCollisionIndex == i {
				continue
			}

			tmpName := sources[destCollisionIndex] + ".tmp"
			// while the temporary name for the current file that's in the way is
			// still conflicting with existing files, we keep adding more ".tmp"s to
			// the end
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

			// reorganize the data structures related to the movement of the tmpfile
			delete(sourceCollisionLookup, sources[destCollisionIndex])
			sources[destCollisionIndex] = tmpName
			sourceCollisionLookup[tmpName] = destCollisionIndex
		}

		os.Rename(source, dest)
	}
}