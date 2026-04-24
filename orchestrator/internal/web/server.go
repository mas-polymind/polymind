package web

import (
	"encoding/json"
	"log"
	"net/http"
	"orchestrator/internal/broker"
	"orchestrator/internal/database"
	"sync"
	"time"

	"github.com/gorilla/websocket"
)

// Server представляет HTTP и WebSocket сервер для веб-интерфейса.
type Server struct {
	broker      *broker.Broker
	upgrader    websocket.Upgrader
	clients     map[*websocket.Conn]bool
	clientsLock sync.RWMutex
}

// NewServer создаёт новый экземпляр сервера.
func NewServer(b *broker.Broker) *Server {
	return &Server{
		broker: b,
		upgrader: websocket.Upgrader{
			CheckOrigin: func(r *http.Request) bool {
				return true // Разрешаем все origin для разработки
			},
		},
		clients: make(map[*websocket.Conn]bool),
	}
}

// Start запускает HTTP сервер на указанном адресе.
func (s *Server) Start(addr string) error {
	http.HandleFunc("/ws", s.handleWebSocket)
	http.HandleFunc("/api/message", s.handleMessage)
	http.HandleFunc("/health", s.handleHealth)

	log.Printf("Web server starting on %s", addr)
	return http.ListenAndServe(addr, nil)
}

// handleHealth отвечает на health check.
func (s *Server) handleHealth(w http.ResponseWriter, r *http.Request) {
	w.WriteHeader(http.StatusOK)
	w.Write([]byte("OK"))
}

// handleMessage принимает POST запрос с сообщением от пользователя.
func (s *Server) handleMessage(w http.ResponseWriter, r *http.Request) {
	// Разрешаем CORS
	w.Header().Set("Access-Control-Allow-Origin", "*")
	w.Header().Set("Access-Control-Allow-Methods", "POST, OPTIONS")
	w.Header().Set("Access-Control-Allow-Headers", "Content-Type")

	if r.Method == http.MethodOptions {
		w.WriteHeader(http.StatusOK)
		return
	}

	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var req struct {
		Text string `json:"text"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid JSON", http.StatusBadRequest)
		return
	}

	if req.Text == "" {
		http.Error(w, "Text is required", http.StatusBadRequest)
		return
	}

	// Создаём задачу в БД с chatID = 0 (веб-пользователь)
	task, err := database.CreateTask(0, req.Text)
	if err != nil {
		log.Printf("CreateTask error: %v", err)
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}

	// Публикуем в очередь task.scout
	msgBytes, _ := json.Marshal(map[string]uint{"task_id": task.ID})
	if err := s.broker.Publish("task.scout", msgBytes); err != nil {
		log.Printf("Failed to publish scout task %d: %v", task.ID, err)
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}

	// Отправляем ответ с task_id
	resp := map[string]interface{}{
		"task_id": task.ID,
		"status":  "accepted",
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(resp)
}

// handleWebSocket устанавливает WebSocket соединение.
func (s *Server) handleWebSocket(w http.ResponseWriter, r *http.Request) {
	conn, err := s.upgrader.Upgrade(w, r, nil)
	if err != nil {
		log.Printf("WebSocket upgrade error: %v", err)
		return
	}
	defer conn.Close()

	s.clientsLock.Lock()
	s.clients[conn] = true
	s.clientsLock.Unlock()

	log.Printf("New WebSocket client connected")

	// Читаем сообщения от клиента (можно использовать для двусторонней связи)
	for {
		messageType, p, err := conn.ReadMessage()
		if err != nil {
			log.Printf("WebSocket read error: %v", err)
			break
		}
		if messageType == websocket.TextMessage {
			log.Printf("Received WebSocket message: %s", p)
			// Можно обработать, например, ping/pong
		}
	}

	s.clientsLock.Lock()
	delete(s.clients, conn)
	s.clientsLock.Unlock()
	log.Printf("WebSocket client disconnected")
}

// Broadcast отправляет сообщение всем подключённым WebSocket клиентам.
func (s *Server) Broadcast(msg interface{}) {
	data, err := json.Marshal(msg)
	if err != nil {
		log.Printf("Broadcast marshal error: %v", err)
		return
	}

	s.clientsLock.RLock()
	defer s.clientsLock.RUnlock()

	for client := range s.clients {
		err := client.WriteMessage(websocket.TextMessage, data)
		if err != nil {
			log.Printf("Broadcast write error: %v", err)
			client.Close()
			delete(s.clients, client)
		}
	}
}

// SendToTask отправляет сообщение клиентам, подписанным на конкретный task_id.
// В данной реализации просто broadcast, но можно расширить.
func (s *Server) SendToTask(taskID uint, msg string) {
	s.Broadcast(map[string]interface{}{
		"task_id": taskID,
		"message": msg,
		"time":    time.Now().Unix(),
	})
}