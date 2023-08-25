package main

import (
	"log"
	"net/http"
	"os"

	"github.com/gorilla/websocket"
)

var channelA = make(chan []byte)
var channelB = make(chan []byte)

var portA *websocket.Conn
var portB *websocket.Conn

var upgrader = websocket.Upgrader{CheckOrigin: func(r *http.Request) bool { return true }}

func relayHandler(name string, inbox chan []byte, outbox chan []byte) func(w http.ResponseWriter, r *http.Request) {
	var openWebSockets = make([]*websocket.Conn, 0, 2)

	return func(w http.ResponseWriter, r *http.Request) {
		for i, v := range openWebSockets {
			if v == nil {
				continue
			}
			log.Println("Closing old connection.", name, i)
			if err := v.WriteMessage(websocket.CloseMessage, websocket.FormatCloseMessage(websocket.CloseNormalClosure, "")); err != nil {
				log.Println("Error in WriteMessage:", err, name, i)
			}
			v.Close()
		}
		openWebSockets = openWebSockets[:0]

		log.Println("Entering localHandler.", name)
		conn, err := upgrader.Upgrade(w, r, nil)
		if err != nil {
			log.Println(err)
			return
		}
		openWebSockets = append(openWebSockets, conn)
		conn.SetReadLimit(1024 * 64)

		log.Println("New Relay Connection, IP:", r.RemoteAddr, name)

		go func() {
			for {
				payload, ok := <-outbox
				if !ok {
					log.Println("Channel closed:", err, name)
					return
				}
				if conn == nil {
					outbox <- payload
					return
				}
				log.Println("Relay Message Out:", name, string(payload))
				err = conn.WriteMessage(websocket.TextMessage, payload)
				if err != nil {
					log.Println("Error in WriteMessage:", err, name)
					return
				}
			}
		}()

		for {
			_, payload, err := conn.ReadMessage()
			if err != nil {
				log.Println("Error in ReadMessage:", err, name)
				break
			}
			log.Println("Relay Message In:", name, string(payload))
			inbox <- payload
		}

		log.Println("Exiting localHandler.")
		conn.Close()
		conn = nil
	}
}

func main() {

	var port = os.Getenv("PORT")
	if port == "" {
		port = "8081"
	}

	http.HandleFunc("/portA", relayHandler("portA", channelA, channelB))
	http.HandleFunc("/portB", relayHandler("portB", channelB, channelA))

	log.Println("HTTP is listening on port " + port)
	log.Fatal(http.ListenAndServe(":"+port, nil))
}
