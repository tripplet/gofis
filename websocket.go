package main

import (
	"log"
	"net/http"
	"sync"

	"github.com/gorilla/websocket"
	"github.com/rjeczalik/notify"
)

var upgrader = websocket.Upgrader{
	ReadBufferSize:  1024,
	WriteBufferSize: 1024,
}

var mutex = &sync.Mutex{}
var websockets []chan *notify.EventInfo

// ws handles websocket requests
func ws(w http.ResponseWriter, r *http.Request) {
	conn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		log.Println(err)
		return
	}

	//log.Println("Websocket client connected")

	closeconn := make(chan bool)
	events := registerWsClient()

	conn.SetCloseHandler(func(code int, text string) error {
		//log.Println("Websocket closing")
		closeconn <- true
		return nil
	})

	connectionActive := true
	for connectionActive {
		select {
		case <-closeconn:
			//log.Println("Websocket closed")
			connectionActive = false

		case event := <-events:
			msg := (*event).Event().String() + ": " + (*event).Path()
			if err := conn.WriteMessage(websocket.TextMessage, []byte(msg)); err != nil {
				log.Println(err)
				connectionActive = false
			}
		}
	}

	removeWsClient(events)
}

func registerWsClient() chan *notify.EventInfo {
	events := make(chan *notify.EventInfo)

	mutex.Lock()
	websockets = append(websockets, events)
	mutex.Unlock()

	return events
}

func removeWsClient(eventChannel chan *notify.EventInfo) {
	mutex.Lock()
	for i := range websockets {
		if websockets[i] == eventChannel {
			websockets = append(websockets[:i], websockets[i+1:]...)
			break
		}
	}
	mutex.Unlock()
}

func startNotifyWsClients(eventChannel chan notify.EventInfo) {
	go func() {
		for {
			evt := <-eventChannel
			log.Println("Filesystem event:", evt)
			mutex.Lock()
			for _, client := range websockets {
				client <- &evt
			}
			mutex.Unlock()
		}
	}()
}
