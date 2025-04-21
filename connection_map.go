package main

import (
	"github.com/dgrr/websocket"
)

// connToPassphrase maps WebSocket connection IDs to passphrases
var connToPassphrase = make(map[uint64]string)

// registerConnection associates a connection with a session passphrase
func registerConnection(conn *websocket.Conn, passphrase string) {
	if conn != nil {
		connToPassphrase[conn.ID()] = passphrase
	}
}

// getPassphraseForConn retrieves the passphrase associated with a connection
func getPassphraseForConn(conn *websocket.Conn) (string, bool) {
	if conn == nil {
		return "", false
	}
	passphrase, found := connToPassphrase[conn.ID()]
	return passphrase, found
}

// unregisterConnection removes a connection from the mapping
func unregisterConnection(conn *websocket.Conn) {
	if conn != nil {
		delete(connToPassphrase, conn.ID())
	}
}
