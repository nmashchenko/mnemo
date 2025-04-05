package ws

import (
	"encoding/json"
	"fmt"
	"github.com/gorilla/websocket"
	"github.com/pkg/errors"
	"log"
	"net/http"
	"sync"
	"time"
)

// note: hack for now (this is stupid)
const professorAPIKey = "my_secret_key"

var upgrader = websocket.Upgrader{
	ReadBufferSize:  1024,
	WriteBufferSize: 1024,
	//CheckOrigin: checkOrigin
}

type Manager struct {
	clients  ClientList
	sync     sync.RWMutex
	handlers map[string]EventHandler
}

func NewManager() *Manager {
	m := &Manager{
		clients:  make(ClientList),
		handlers: make(map[string]EventHandler),
	}

	m.setupEventHandlers()
	return m
}

func (m *Manager) setupEventHandlers() {
	m.handlers[EventSendMessage] = SendMessage
}

func SendMessage(event Event, c *Client) error {
	var chatEvent SendMessageEvent

	if err := json.Unmarshal(event.Payload, &chatEvent); err != nil {
		return fmt.Errorf("bad payload in request: %v", err)
	}

	var broadcastMessage NewMessageEvent
	broadcastMessage.Sent = time.Now()
	broadcastMessage.Message = chatEvent.Message
	broadcastMessage.From = chatEvent.From

	data, err := json.Marshal(broadcastMessage)
	if err != nil {
		return fmt.Errorf("failed to marshal broadcast message: %v", err)
	}

	outgoingEvent := Event{
		Payload: data,
		Type:    EventNewMessage,
	}

	// acquire read lock to safely iterate over the clients map.
	c.manager.sync.RLock()
	defer c.manager.sync.RUnlock()

	if c.role == RoleProfessor {
		// case 1: professor should be able to stream questions to students
		for client := range c.manager.clients {
			if client.role == RoleStudent {
				sendWithRetry(client.egress, outgoingEvent)
			}
		}
	} else if c.role == RoleStudent {
		// students should only stream back responses to professor
		for client := range c.manager.clients {
			if client.role == RoleProfessor {
				sendWithRetry(client.egress, outgoingEvent)
			}
		}
	} else {
		return fmt.Errorf("unknown client role: %s", c.role)
	}

	return nil
}

func (m *Manager) routeEvent(event Event, c *Client) error {
	// check if the event type is part of the handlers
	if handler, ok := m.handlers[event.Type]; ok {
		if err := handler(event, c); err != nil {
			return err
		}

		return nil
	} else {
		return errors.New("there is no such event type")
	}
}

func (m *Manager) ServeWs(w http.ResponseWriter, r *http.Request) {
	conn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		log.Println(err)
		return
	}

	// Check for x-api-key header. If it matches, assign professor role.
	apiKey := r.Header.Get("x-api-key")
	role := RoleStudent
	if apiKey == professorAPIKey {
		role = RoleProfessor
	}

	client := NewClient(conn, m, role)

	m.addClient(client)
	fmt.Printf("%s connected\n", client.role)

	go client.readMessages()
	go client.writeMessages()
}

func (m *Manager) addClient(client *Client) {
	m.sync.Lock()
	defer m.sync.Unlock()

	m.clients[client] = true
}

func (m *Manager) removeClient(client *Client) {
	m.sync.Lock()
	defer m.sync.Unlock()

	if _, ok := m.clients[client]; ok {
		client.connection.Close()
		delete(m.clients, client)
	}
}
