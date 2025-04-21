package main

import (
	"sync"

	"github.com/dgrr/websocket"
)

// connToPassphrase maps WebSocket connection IDs to passphrases
var connToPassphrase = make(map[uint64]string)
var connMutex sync.Mutex

// registerConnection associates a connection with a session passphrase
func registerConnection(conn *websocket.Conn, passphrase string) {
	if conn != nil {
		connMutex.Lock()
		connToPassphrase[conn.ID()] = passphrase
		connMutex.Unlock()
	}
}

// getPassphraseForConn retrieves the passphrase associated with a connection
func getPassphraseForConn(conn *websocket.Conn) (string, bool) {
	if conn == nil {
		return "", false
	}
	connMutex.Lock()
	defer connMutex.Unlock()
	passphrase, found := connToPassphrase[conn.ID()]
	return passphrase, found
}

// unregisterConnection removes a connection from the mapping
func unregisterConnection(conn *websocket.Conn) {
	if conn != nil {
		connMutex.Lock()
		delete(connToPassphrase, conn.ID())
		connMutex.Unlock()
	}
}
