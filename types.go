package main

import "github.com/dgrr/websocket"

type session struct {
	id         uint64
	passphrase string
	sdp        string
	sConn      *websocket.Conn
	rConn      *websocket.Conn
}

type messageType string

const (
	offerType             messageType = "offer"
	answerType            messageType = "answer"
	iceCandidateType      messageType = "ice-candidate"
	connectionRequestType messageType = "connection-request"
	passphraseType        messageType = "passphrase"
)

type signalingMessage struct {
	Type       messageType
	Passphrase *string
	Payload    *string
}
