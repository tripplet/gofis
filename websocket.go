package main

import (
	"encoding/json"
	"log"
	"net/http"
	"path/filepath"
	"sync"

	"github.com/gorilla/websocket"
	"github.com/rjeczalik/notify"
)

var upgrader = websocket.Upgrader{
	ReadBufferSize:  1024,
	WriteBufferSize: 1024,
}

type fileEvent struct {
	EventType string `json:"event_type"`
	Path      string `json:"path"`
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

	closeconn := make(chan bool)
	events := registerWsClient()

	conn.SetCloseHandler(func(code int, text string) error {
		closeconn <- true
		return nil
	})

	connectionActive := true
	for connectionActive {
		select {
		case <-closeconn:
			connectionActive = false

		case event := <-events:
			path, err := filepath.Rel(*basePath, (*event).Path())
			if err != nil {
				continue
			}

			eventMsg := &fileEvent{EventType: (*event).Event().String(), Path: path}
			eventMsgJSON, _ := json.Marshal(eventMsg)

			if err := conn.WriteMessage(websocket.TextMessage, []byte(eventMsgJSON)); err != nil {
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
