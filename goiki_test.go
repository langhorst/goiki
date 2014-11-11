package main

import (
	"testing"
)

func TestProcessLinks(t *testing.T) {
	originals := [][]byte{
		[]byte("Life is like riding a [bicycle]()."),
		[]byte("[Life]() is like riding a [bicycle]()."),
		[]byte("To keep your [balance]() you must [keep]() your [balance]()."),
	}

	results := [][]byte{
		[]byte("Life is like riding a [bicycle](bicycle)."),
		[]byte("[Life](Life) is like riding a [bicycle](bicycle)."),
		[]byte("To keep your [balance](balance) you must [keep](keep) your [balance](balance)."),
	}

	for i := 0; i < len(originals); i++ {
		processed := processLinks(originals[i])
		if string(processed) != string(results[i]) {
			t.Errorf("Expected >%s<, got >%s<\n", results[i], processed)
		}
	}
}
