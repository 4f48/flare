package main

import (
	"github.com/dgrr/websocket"
)

type Session struct {
	Id         uint64
	Passphrase string
	SDP        string
	sConn      *websocket.Conn
	rConn      *websocket.Conn
	iceBuffer  []string
}

type messageType string

const (
	offerType        messageType = "offer"
	answerType       messageType = "answer"
	iceCandidateType messageType = "ice-candidate"
	connReqType      messageType = "connection-request"
	passphraseType   messageType = "passphrase"
)

type signalingMessage struct {
	Type          messageType `json:"type"`
	PassphraseLen uint8       `json:"passphraseLength"`
	SDP           string      `json:"sdp"`
	Passphrase    string      `json:"passphrase"`
	Candidate     string      `json:"candidate"`
}

type OfferMsg struct {
	Type          messageType `json:"type"`
	PassphraseLen int         `json:"passphraseLength"`
	SDP           string      `json:"sdp"`
}

type ConnReqMsg struct {
	Type       messageType `json:"type"`
	Passphrase string      `json:"passphrase"`
	SDP        string      `json:"sdp"`
}

type PassphraseMsg struct {
	Type       messageType `json:"type"`
	Passphrase string      `json:"passphrase"`
}

type AnswerMsg struct {
	Type messageType `json:"type"`
	SDP  string      `json:"sdp"`
}

type IceCandidateMsg struct {
	Type      messageType `json:"type"`
	Candidate string      `json:"candidate"`
}
