package main

import (
	"crypto/rand"
	"errors"
	"fmt"
	"log"
	"os"
	"os/signal"

	"github.com/bytedance/sonic"
	"github.com/dgrr/websocket"
	"github.com/valyala/fasthttp"
)

const port = "8080"

var wordlist, wlerr = readWordlist("eff_large_wordlist.txt")

var sessions = make(map[string]Session)

func init() {
	if wlerr != nil {
		panic(wlerr.Error())
	}
}

func main() {
	ws := websocket.Server{}
	ws.HandleData(dataHandler)
	ws.HandleClose(disconnectHandler)

	server := fasthttp.Server{
		Handler: ws.Upgrade,
	}
	go server.ListenAndServe(fmt.Sprintf(":%s", port))
	log.Printf("WebSocket server listening on 0.0.0.0:%s", port)

	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, os.Interrupt)
	<-sigCh
	signal.Stop(sigCh)
	signal.Reset(os.Interrupt)

	log.Print("Shutting down...")
	server.Shutdown()
}

func dataHandler(conn *websocket.Conn, isBin bool, data []byte) {
	var msg signalingMessage

	err := sonic.Unmarshal(data, &msg)
	if err != nil {
		conn.CloseDetail(websocket.StatusNotAcceptable, "invalid JSON")
		log.Printf("Failed to unmarshal JSON data: %s", err)
		return
	}

	log.Printf("Request from %s of type %s", conn.RemoteAddr(), msg.Type)

	switch msg.Type {
	case offerType:
		go func() {
			err := handleOffer(data, conn)
			if err != nil {
				conn.CloseDetail(websocket.StatusProtocolError, err.Error())
				log.Print(err)
			}
		}()
		break
	case connReqType:
		go func() {
			err := handleConnReq(data, conn)
			if err != nil {
				conn.CloseDetail(websocket.StatusProtocolError, fmt.Sprintf("error: %s", err))
				log.Print(err)
			}
		}()
		break
	case answerType:
		go func() {
			err := handleAnswer(data)
			if err != nil {
				conn.CloseDetail(websocket.StatusProtocolError, fmt.Sprintf("error: %s", err))
				log.Print(err)
			}
		}()
		break
	case iceCandidateType:
		go func() {
			err := handleIceCandidate(data, conn)
			if err != nil {
				conn.CloseDetail(websocket.StatusProtocolError, fmt.Sprintf("error: %s", err))
				log.Print(err)
			}
		}()
		break
	default:
		conn.CloseDetail(websocket.StatusNotAcceptable, "invalid message")
		log.Printf("Invalid signalingMessage: %s", msg.Type)
	}
}

func disconnectHandler(conn *websocket.Conn, err error) {
	log.Printf("Connection from %s closed: %v", conn.RemoteAddr(), err)
	go func() {
		for passphrase, session := range sessions {
			if session.sConn == conn || session.rConn == conn {
				if session.sConn == conn {
					if session.rConn != nil {
						session.rConn.CloseDetail(websocket.StatusGoAway, "sender disconnected")
					}
				}
				if session.rConn == conn {
					session.rConn.CloseDetail(websocket.StatusGoAway, "receiver disconnected")
				}

				delete(sessions, passphrase)
				log.Printf("Session %s closed due to client disconnect", passphrase)
				break
			}
		}
	}()
}

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
		return errors.New("session not found")
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

func handleAnswer(data []byte) error {
	var msg AnswerMsg
	err := sonic.Unmarshal(data, &msg)
	if err != nil {
		return err
	}
	if msg.Type != answerType {
		return errors.New("invalid answer")
	}
	session, found := sessions[msg.Passphrase]
	if !found {
		return errors.New("session not found")
	}

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
	session, found := sessions[msg.Passphrase]
	if !found {
		return errors.New("session not found")
	}

	// If sender connection is sending the candidate
	if session.sConn == conn {
		// If receiver connection isn't established yet, buffer the candidate
		if session.rConn == nil {
			// Store the candidate in the buffer
			session.iceBuffer = append(session.iceBuffer, msg.Candidate)
			sessions[msg.Passphrase] = session
			return nil
		}

		// If receiver is connected, forward the candidate
		ans, err := sonic.Marshal(IceCandidateMsg{
			Type:      iceCandidateType,
			Candidate: msg.Candidate,
		})
		if err != nil {
			return err
		}
		_, err = session.rConn.Write(ans)
		return err
	} else if session.rConn == conn {
		// If receiver is sending a candidate, forward to sender
		ans, err := sonic.Marshal(IceCandidateMsg{
			Type:      iceCandidateType,
			Candidate: msg.Candidate,
		})
		if err != nil {
			return err
		}
		_, err = session.sConn.Write(ans)
		return err
	}

	return errors.New("stranger connection")
}
