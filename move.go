package main

type moveFunc func(src, dst string) error

func moveAll(srcToDst map[string]string, m moveFunc, t tmpFunc) error {
	for src, dst := range srcToDst {
		if src == dst {
			delete(srcToDst, src)
			continue
		}

		err := moveRec(srcToDst, m, t, map[string]struct{}{}, src, dst)
		if err != nil {
			return err
		}
		delete(srcToDst, src)
	}
	// as 3. under https://go.dev/ref/spec#For_range indicates, insertions into
	// maps during range iterations over them are not guaranteed to be
	// produced, therefore, we must do another iteration, and in this one,
	// everything is guaranteed to require only one move because we know that
	// whatever occupied the destinations have already been moved in the
	// previous loop
	for src, dst := range srcToDst {
		err := m(src, dst)
		if err != nil {
			return err
		}
		delete(srcToDst, src)
	}

	return nil
}

func moveRec(srcToDst map[string]string, m moveFunc, t tmpFunc, seen map[string]struct{}, src, dst string) error {
	otherSrc := dst
	otherDst, found := srcToDst[otherSrc]
	if found {
		// there is currently a file in the way of the destination

		_, seenOther := seen[otherSrc]
		if seenOther {
			// the file that's in the way has already been seen, so there is a
			// cycle, meaning we'll have to make an additional temporary move

			tmpSrc, err := t(src)
			if err != nil {
				return err
			}

			err = m(src, tmpSrc)
			if err != nil {
				return err
			}

			srcToDst[tmpSrc] = dst
			return nil
		}

		// track that we've seen this; seen never used after a recursive call
		// within the isolated iteration, so we don't need to worry about it
		// being mutated by child calls
		seen[src] = struct{}{}
		err := moveRec(srcToDst, m, t, seen, otherSrc, otherDst)
		if err != nil {
			return err
		}
		delete(srcToDst, otherSrc)
	}

	return m(src, dst)
}
