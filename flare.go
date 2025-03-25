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

var sessions = make(map[string]session)

func init() {
	if wlerr != nil {
		panic(wlerr.Error())
	}
}

func main() {
	ws := websocket.Server{}
	ws.HandleData(dataHandler)

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

	switch msg.Type {
	case offerType:
		go func() {
			err := createOffer(msg, conn)
			if err != nil {
				conn.CloseDetail(websocket.StatusProtocolError, err.Error())
				log.Print(err)
			}
		}()
		break
	case connectionRequestType:
		go func() {
			err := processConnReq(msg, conn)
			if err != nil {
				conn.CloseDetail(websocket.StatusProtocolError, fmt.Sprintf("error: %s", err))
				log.Print(err)
			}
		}()
		break
	case answerType:
		go func() {
			err := processAnswer(msg)
			if err != nil {
				conn.CloseDetail(websocket.StatusProtocolError, fmt.Sprintf("error: %s", err))
				log.Print(err)
			}
		}()
		break
	case iceCandidateType:
		go func() {
			err := forwardIceCandidates(msg, conn)
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

func createOffer(msg signalingMessage, conn *websocket.Conn) error {
	passphrase, err := generatePassphrase(6, wordlist, rand.Reader)
	if err != nil {
		return err
	}
	sessions[passphrase] = session{
		id:         conn.ID(),
		passphrase: passphrase,
		sdp:        *msg.Payload,
		sConn:      conn,
	}

	ans, err := sonic.Marshal(signalingMessage{
		Type:       "passphrase",
		Passphrase: &passphrase,
	})
	if err != nil {
		return err
	}
	_, err = conn.Write([]byte(ans))
	return err
}

func processConnReq(msg signalingMessage, conn *websocket.Conn) error {
	sess, found := sessions[*msg.Passphrase]
	if !found {
		return errors.New("session not found")
	}
	ans, err := sonic.Marshal(signalingMessage{
		Type:    "offer",
		Payload: &sess.sdp,
	})
	if err != nil {
		return err
	}
	_, err = conn.Write([]byte(ans))

	sessions[*msg.Passphrase] = session{
		id:         sess.id,
		passphrase: sess.passphrase,
		sConn:      sess.sConn,
		rConn:      conn,
	}
	return err
}

func processAnswer(msg signalingMessage) error {
	sess, found := sessions[*msg.Passphrase]
	if !found {
		return errors.New("session not found")
	}
	ans, err := sonic.Marshal(signalingMessage{
		Type:    "answer",
		Payload: msg.Payload,
	})
	if err != nil {
		return err
	}
	_, err = sess.sConn.Write([]byte(ans))
	return err
}

func forwardIceCandidates(msg signalingMessage, conn *websocket.Conn) error {
	sess, found := sessions[*msg.Passphrase]
	if !found {
		return errors.New("session not found")
	}

	ans, err := sonic.Marshal(signalingMessage{
		Type:    "ice-candidate",
		Payload: msg.Payload,
	})
	if err != nil {
		return err
	}

	if sess.sConn == conn {
		sess.rConn.Write([]byte(ans))
	} else if sess.rConn == conn {
		sess.sConn.Write([]byte(ans))
	} else {
		return errors.New("stranger connection")
	}

	return nil
}
