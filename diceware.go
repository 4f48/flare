package main

import (
	"bufio"
	"crypto/rand"
	"errors"
	"fmt"
	"io"
	"math/big"
	"os"
	"strings"
)

var max *big.Int = big.NewInt(int64(len(wordlist)))

func readWordlist(fileName string) ([]string, error) {
	file, err := os.Open(fileName)
	if err != nil {
		return nil, err
	}
	defer file.Close()

	scanner := bufio.NewScanner(file)
	wordlist := make([]string, 7776) // assumes the EFF's large word list
	for i := range wordlist {
		scanner.Scan()
		line := scanner.Text()
		_, word, found := strings.Cut(line, "\t")
		if !found {
			return nil, errors.New(fmt.Sprintf("unexpected line in diceware file at index %d: %s", i, line))
		}
		wordlist[i] = word
	}
	return wordlist, scanner.Err()
}

func generatePassphrase(length uint8, wordlist []string, randSrc io.Reader) (string, error) {
	words := make([]string, length)
	for i := range length {
		idx, err := rand.Int(randSrc, max)
		if err != nil {
			return "", err
		}
		words[i] = wordlist[idx.Uint64()]
	}
	return strings.Join(words, "-"), nil
}
