package main

import (
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
			err := handleAnswer(data, conn) // Pass the connection
			if err != nil {
				conn.CloseDetail(websocket.StatusProtocolError, fmt.Sprintf("error: %s", err))
				log.Print(err)
			}
		}()
		break
	case iceCandidateType:
		go func() {
			err := handleIceCandidate(data, conn) // Pass the connection
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
		// First check if the connection has a passphrase
		passphrase, found := getPassphraseForConn(conn)
		if found {
			// If found directly, we can look up the session and clean up
			sessionsMutex.Lock()
			session, exists := sessions[passphrase]
			if exists {
				if session.sConn == conn {
					if session.rConn != nil {
						session.rConn.CloseDetail(websocket.StatusGoAway, "sender disconnected")
						unregisterConnection(session.rConn)
					}
				} else if session.rConn == conn {
					if session.sConn != nil {
						session.sConn.CloseDetail(websocket.StatusGoAway, "receiver disconnected")
						unregisterConnection(session.sConn)
					}
				}
				delete(sessions, passphrase)
			}
			sessionsMutex.Unlock()

			// Unregister this connection
			unregisterConnection(conn)
		} else {
			// If not found in our mapping, we need to scan all sessions
			sessionsMutex.Lock()
			for passphrase, session := range sessions {
				if session.sConn == conn || session.rConn == conn {
					if session.sConn == conn {
						unregisterConnection(session.sConn)
						if session.rConn != nil {
							session.rConn.CloseDetail(websocket.StatusGoAway, "sender disconnected")
							unregisterConnection(session.rConn)
						}
					}
					if session.rConn == conn {
						unregisterConnection(session.rConn)
						if session.sConn != nil {
							session.sConn.CloseDetail(websocket.StatusGoAway, "sender disconnected")
							unregisterConnection(session.sConn)
						}
					}
					delete(sessions, passphrase)
					break
				}
			}
			sessionsMutex.Unlock()
		}
	}()
}
