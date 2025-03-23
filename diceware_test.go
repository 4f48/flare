package main

import (
	"crypto/rand"
	"log"
	"os"
	"os/signal"
	"testing"
)

func TestGeneratePassphrase(t *testing.T) {
	wordlist, err := readWordlist("eff_large_wordlist.txt")
	if err != nil {
		panic(err)
	}

	running := true
	go func() {
		for running {
			passphrase, err := generatePassphrase(6, wordlist, rand.Reader)
			if err != nil {
				panic(err)
			}
			log.Print(passphrase)
		}
	}()

	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, os.Interrupt)
	<-sigCh
	signal.Stop(sigCh)
	signal.Reset(os.Interrupt)
	running = false
}
