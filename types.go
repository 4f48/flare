package main

type session struct {
	id         uint64
	passphrase string
	sdp        string
}

type messageType string

const (
	offerType             messageType = "offer"
	answerType            messageType = "answer"
	iceCandidateType      messageType = "ice-candidate"
	connectionRequestType messageType = "connection-request"
)

type signalingMessage struct {
	Type       messageType
	Passphrase *string
	Payload    *string
}
