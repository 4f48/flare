package main

import (
	"crypto/rand"
	"errors"
	"fmt"
	"log"

	"github.com/bytedance/sonic"
	"github.com/dgrr/websocket"
)

func handleOffer(data []byte, conn *websocket.Conn) error {
	var msg OfferMsg
	err := sonic.Unmarshal(data, &msg)
	if err != nil {
		return err
	}
	if msg.Type != offerType {
		return errors.New("invalid offer")
	}

	passphrase, err := generatePassphrase(6, wordlist, rand.Reader)
	if err != nil {
		return err
	}
	sessions[passphrase] = Session{
		Id:         conn.ID(),
		Passphrase: passphrase,
		SDP:        msg.SDP,
		sConn:      conn,
		iceBuffer:  make([]string, 0),
	}

	registerConnection(conn, passphrase)

	ans, err := sonic.Marshal(PassphraseMsg{
		Type:       passphraseType,
		Passphrase: passphrase,
	})
	if err != nil {
		return err
	}
	_, err = conn.Write([]byte(ans))
	return err
}

func handleConnReq(data []byte, conn *websocket.Conn) error {
	var msg ConnReqMsg
	err := sonic.Unmarshal(data, &msg)
	if err != nil {
		return err
	}
	if msg.Type != connReqType {
		return errors.New("invalid connection request")
	}
	session, found := sessions[msg.Passphrase]
	if !found {
		return errors.New(fmt.Sprintf("session not found: %s", msg.Passphrase))
	}

	ans, err := sonic.Marshal(OfferMsg{
		Type: offerType,
		SDP:  session.SDP,
	})
	if err != nil {
		return err
	}
	_, err = conn.Write([]byte(ans))

	session.rConn = conn
	sessions[msg.Passphrase] = session

	registerConnection(conn, msg.Passphrase)

	for _, candidate := range session.iceBuffer {
		candidateMsg, err := sonic.Marshal(IceCandidateMsg{
			Type:      iceCandidateType,
			Candidate: candidate,
		})
		if err != nil {
			log.Printf("Error marshaling buffered ICE candidate: %v", err)
			continue
		}

		_, err = conn.Write(candidateMsg)
		if err != nil {
			log.Printf("Error sending buffered ICE candidate: %v", err)
		}
	}
	return err
}

func handleAnswer(data []byte, conn *websocket.Conn) error {
	var msg AnswerMsg
	err := sonic.Unmarshal(data, &msg)
	if err != nil {
		return err
	}
	if msg.Type != answerType {
		return errors.New("invalid answer")
	}

	// Get passphrase from connection mapping
	passphrase, found := getPassphraseForConn(conn)
	if !found {
		return errors.New("session not found for connection")
	}

	session, found := sessions[passphrase]
	if !found {
		return errors.New("session not found")
	}

	// Forward the answer to the sender
	_, err = session.sConn.Write(data)
	return err
}

func handleIceCandidate(data []byte, conn *websocket.Conn) error {
	var msg IceCandidateMsg
	err := sonic.Unmarshal(data, &msg)
	if err != nil {
		return err
	}
	if msg.Type != iceCandidateType {
		return errors.New("invalid ice-candidate message")
	}

	// Get passphrase from connection mapping
	passphrase, found := getPassphraseForConn(conn)
	if !found {
		return errors.New("session not found for connection")
	}

	session, found := sessions[passphrase]
	if !found {
		return errors.New("session not found")
	}

	// If sender connection is sending the candidate
	if session.sConn == conn {
		// If receiver connection isn't established yet, buffer the candidate
		if session.rConn == nil {
			// Store the candidate in the buffer
			session.iceBuffer = append(session.iceBuffer, msg.Candidate)
			sessions[passphrase] = session
			return nil
		}

		// If receiver is connected, forward the candidate
		_, err = session.rConn.Write(data)
		return err
	} else if session.rConn == conn {
		// If receiver is sending a candidate, forward to sender
		_, err = session.sConn.Write(data)
		return err
	}

	return errors.New("stranger connection")
}
