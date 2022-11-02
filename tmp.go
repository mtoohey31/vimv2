package main

import (
	"fmt"
	"math/rand"
)

type tmpFunc func(src string) (tmpSrc string, err error)

func tmpClosure[T, U any](s1 map[string]T, s2 map[string]U) tmpFunc {
	return func(src string) (string, error) {
		for i := 0; i < 10000; i++ {
			tmpSrc := fmt.Sprintf("%s.tmp%d", src, rand.Int())
			_, found1 := s1[tmpSrc]
			_, found2 := s2[tmpSrc]
			if !(found1 || found2) {
				return tmpSrc, nil
			}
		}

		return "", fmt.Errorf("failed to find temporary location for %s", src)
	}
}
