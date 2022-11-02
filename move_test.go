package main

import (
	"fmt"
	"testing"
)

func Test_moveAll(t *testing.T) {
	tests := []map[string]string{
		{
			// no moves
			"a": "a",
			"b": "b",
			"c": "c",
		},
		{
			// some moves
			"a": "a",
			"b": "c",
			"d": "e",
		},
		{
			// all move
			"a": "b",
			"c": "d",
			"e": "f",
		},
		{
			// chain
			"a": "b",
			"b": "c",
			"c": "d",
		},
		{
			// single cycle
			"a": "b",
			"b": "c",
			"c": "a",
		},
		{
			// triple cycle
			"a": "b",
			"b": "c",
			"c": "a",

			"1": "2",
			"2": "3",
			"3": "1",

			"x": "y",
			"y": "z",
			"z": "x",
		},
	}

	for _, test := range tests {
		t.Run(fmt.Sprintf("%v", test), func(t *testing.T) {
			expected := map[string]string{}
			for src, dst := range test {
				expected[dst] = src
			}

			actual := map[string]string{}
			for src := range test {
				actual[src] = src
			}

			moveFn := func(src, dst string) error {
				t.Logf(`move "%s" -> "%s"`, src, dst)

				var ok bool
				actual[dst], ok = actual[src]
				if !ok {
					t.Fatal("src didn't exist")
				}

				delete(actual, src)

				return nil
			}

			actualErr := moveAll(test, moveFn, tmpClosure(actual, expected))

			if actualErr != nil {
				t.Fatal(actualErr)
			}
			assertMapsEqual(t, expected, actual)
		})
	}
}

func assertMapsEqual[T, U comparable](t *testing.T, expected, actual map[T]U) {
	t.Helper()

	if len(expected) != len(actual) {
		t.Fatalf("expected map: %v and actual map: %v contain "+
			"a different number of values", expected, actual)
	}
	for ek, ev := range expected {
		av, ok := actual[ek]
		if !ok {
			t.Fatalf("actual map: %v didn't contain expected key %v", actual, ek)
		}
		if ev != av {
			t.Fatalf("expected value: %v and actual value: %v differ for key %v",
				ev, av, ek)
		}
	}
}
