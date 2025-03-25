package main

import (
	"crypto/rand"
	"errors"
	"fmt"
	"log"
	"os"
	"os/signal"
	"time"

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
			}
		}()
	case connectionRequestType:
		go func() {
			err := createConnReq(msg, conn)
			if err != nil {
				conn.CloseDetail(websocket.StatusProtocolError, fmt.Sprintf("error: %s", err))
			}
		}()
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
	sessions[fmt.Sprintf("%s:offer", passphrase)] = session{
		id:         conn.ID(),
		passphrase: passphrase,
		sdp:        *msg.Payload,
	}
	conn.Write([]byte(passphrase))
	for range 120 {
		session, found := sessions[fmt.Sprintf("%v:answer", msg.Passphrase)]
		if found {
			conn.Write([]byte(session.sdp))
			return nil
		}
		time.Sleep(1 * time.Second)
	}
	delete(sessions, passphrase)
	return errors.New("timed out waiting for answer")
}

func createConnReq(msg signalingMessage, conn *websocket.Conn) error {
	sessions[fmt.Sprintf("%v:request", msg.Passphrase)] = session{
		id:         conn.ID(),
		passphrase: *msg.Passphrase,
	}
	offer, found := sessions[fmt.Sprintf("%v:offer", msg.Passphrase)]
	if !found {
		return errors.New("session not found")
	}
	_, err := conn.Write([]byte(offer.sdp))
	return err
}
