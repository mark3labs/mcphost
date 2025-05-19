package cmd

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"sync"
	"time"

	"github.com/charmbracelet/log"
	"github.com/google/uuid"
	"github.com/gorilla/mux"
	"github.com/mark3labs/mcphost/pkg/history"
	"github.com/mark3labs/mcphost/pkg/llm"
)

type Conversation struct {
	ID           string                   `json:"id"`
	Messages     []history.HistoryMessage `json:"messages"`
	LastActivity time.Time                `json:"lastActivity"`
}

type ConversationStore struct {
	mu            sync.RWMutex
	conversations map[string]*Conversation
}

func NewConversationStore() *ConversationStore {
	return &ConversationStore{
		conversations: make(map[string]*Conversation),
	}
}

func (s *ConversationStore) GetConversation(id string) (*Conversation, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	conv, ok := s.conversations[id]
	return conv, ok
}

func (s *ConversationStore) CreateConversation() *Conversation {
	s.mu.Lock()
	defer s.mu.Unlock()

	id := uuid.New().String()
	conv := &Conversation{
		ID:           id,
		Messages:     []history.HistoryMessage{},
		LastActivity: time.Now(),
	}

	s.conversations[id] = conv
	return conv
}

// UpdateConversation met à jour une conversation existante
func (s *ConversationStore) UpdateConversation(id string, messages []history.HistoryMessage) bool {
	s.mu.Lock()
	defer s.mu.Unlock()

	if _, exists := s.conversations[id]; !exists {
		return false
	}

	s.conversations[id].Messages = messages
	s.conversations[id].LastActivity = time.Now()
	return true
}

// CloseConversation ferme une conversation
func (s *ConversationStore) CloseConversation(id string) bool {
	s.mu.Lock()
	defer s.mu.Unlock()

	if _, exists := s.conversations[id]; !exists {
		return false
	}

	delete(s.conversations, id)
	return true
}

// StartupCleanupTask démarre une goroutine pour nettoyer les conversations inactives
func (s *ConversationStore) StartupCleanupTask(ctx context.Context) {
	go func() {
		ticker := time.NewTicker(30 * time.Minute)
		defer ticker.Stop()

		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				s.cleanupInactiveConversations()
			}
		}
	}()
}

// cleanupInactiveConversations supprime les conversations inactives depuis plus de 24h
func (s *ConversationStore) cleanupInactiveConversations() {
	s.mu.Lock()
	defer s.mu.Unlock()

	threshold := time.Now().Add(-24 * time.Hour)

	for id, conv := range s.conversations {
		if conv.LastActivity.Before(threshold) {
			delete(s.conversations, id)
			log.Debug("Conversation inactive supprimée", "id", id)
		}
	}
}

// ChatRequest représente une demande de chat
type ChatRequest struct {
	Message     string `json:"message"`
	ReferenceID string `json:"referenceId,omitempty"`
}

// ChatResponse représente la réponse d'une demande de chat
type ChatResponse struct {
	ConversationID string                 `json:"conversationId"`
	Message        history.HistoryMessage `json:"message"`
}

// ServerHandler gère les requêtes HTTP pour le chat
type ServerHandler struct {
	provider      llm.Provider
	tools         []llm.Tool
	store         *ConversationStore
	messageWindow int
}

// NewServerHandler crée un nouveau handler HTTP
func NewServerHandler(provider llm.Provider, tools []llm.Tool, messageWindow int) *ServerHandler {
	return &ServerHandler{
		provider:      provider,
		tools:         tools,
		store:         NewConversationStore(),
		messageWindow: messageWindow,
	}
}

// Setup configure les routes HTTP
func (h *ServerHandler) Setup(ctx context.Context, r *mux.Router) {
	r.HandleFunc("/chat", h.HandleChat).Methods("POST")
	r.HandleFunc("/conversation/{id}", h.HandleCloseConversation).Methods("DELETE")

	// Démarrer la tâche de nettoyage
	h.store.StartupCleanupTask(ctx)
}

// HandleChat traite une requête de chat
func (h *ServerHandler) HandleChat(w http.ResponseWriter, r *http.Request) {
	var req ChatRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Requête invalide", http.StatusBadRequest)
		return
	}

	if req.Message == "" {
		http.Error(w, "Le message ne peut pas être vide", http.StatusBadRequest)
		return
	}

	var conv *Conversation
	var isNewConv bool

	// Conversation existante ou nouvelle
	if req.ReferenceID != "" {
		var exists bool
		conv, exists = h.store.GetConversation(req.ReferenceID)
		if !exists {
			http.Error(w, "Conversation introuvable", http.StatusNotFound)
			return
		}
	} else {
		conv = h.store.CreateConversation()
		isNewConv = true
	}

	// Pruner les messages si nécessaire
	if len(conv.Messages) > h.messageWindow {
		conv.Messages = pruneMessages(conv.Messages)
	}

	// Créer le message utilisateur
	userMessage := history.HistoryMessage{
		Role: "user",
		Content: []history.ContentBlock{
			{
				Type: "text",
				Text: req.Message,
			},
		},
	}

	// Ajouter le message utilisateur
	conv.Messages = append(conv.Messages, userMessage)

	// Appeler l'IA
	err := h.processConversation(r.Context(), &conv.Messages)
	if err != nil {
		http.Error(w, fmt.Sprintf("Erreur lors de l'appel à l'IA: %v", err), http.StatusInternalServerError)
		return
	}

	// Obtenir la réponse
	var aiResponse history.HistoryMessage
	if len(conv.Messages) > 0 {
		for i := len(conv.Messages) - 1; i >= 0; i-- {
			if conv.Messages[i].Role == "assistant" {
				aiResponse = conv.Messages[i]
				break
			}
		}
	}

	// Mettre à jour la conversation
	if !isNewConv {
		h.store.UpdateConversation(conv.ID, conv.Messages)
	}

	// Renvoyer la réponse
	resp := ChatResponse{
		ConversationID: conv.ID,
		Message:        aiResponse,
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(resp)
}

// HandleCloseConversation ferme une conversation
func (h *ServerHandler) HandleCloseConversation(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	id := vars["id"]

	if !h.store.CloseConversation(id) {
		http.Error(w, "Conversation introuvable", http.StatusNotFound)
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

// processConversation traite une conversation avec l'IA
func (h *ServerHandler) processConversation(ctx context.Context, messages *[]history.HistoryMessage) error {
	// Convertir les messages history.HistoryMessage en llm.Message
	var llmMessages []llm.Message
	for _, msg := range *messages {
		llmMessages = append(llmMessages, &msg)
	}

	// Obtenir le texte du dernier message utilisateur
	var prompt string
	if len(*messages) > 0 {
		for i := len(*messages) - 1; i >= 0; i-- {
			if (*messages)[i].Role == "user" {
				prompt = (*messages)[i].GetContent()
				break
			}
		}
	}

	// Envoyer la demande au provider
	message, err := h.provider.CreateMessage(ctx, prompt, llmMessages, h.tools)
	if err != nil {
		return err
	}

	// Traiter la réponse
	msgContent := []history.ContentBlock{
		{
			Type: "text",
			Text: message.GetContent(),
		},
	}

	// Traiter les appels d'outils
	toolCalls := message.GetToolCalls()
	for _, toolCall := range toolCalls {
		// Convertir les arguments en JSON
		var argBytes []byte
		args := toolCall.GetArguments()
		if len(args) > 0 {
			argBytes, err = json.Marshal(args)
			if err != nil {
				log.Error("Erreur de sérialisation des arguments", "error", err)
				continue
			}
		}

		// Ajouter un bloc d'utilisation d'outil
		toolUseBlock := history.ContentBlock{
			Type:  "tool_use",
			ID:    toolCall.GetID(),
			Name:  toolCall.GetName(),
			Input: json.RawMessage(argBytes),
		}
		msgContent = append(msgContent, toolUseBlock)
	}

	// Ajouter le message à l'historique
	*messages = append(*messages, history.HistoryMessage{
		Role:    message.GetRole(),
		Content: msgContent,
	})

	// Traiter les appels d'outils s'il y en a
	if len(toolCalls) > 0 {
		// Dans cette version simplifiée, on simule simplement un résultat vide
		for _, toolCall := range toolCalls {
			// Ajouter le résultat de l'outil (simulation)
			*messages = append(*messages, history.HistoryMessage{
				Role: "tool",
				Content: []history.ContentBlock{
					{
						Type:      "tool_result",
						ToolUseID: toolCall.GetID(),
						Content: []map[string]string{
							{"type": "text", "text": "Résultat de l'outil (simulation)"},
						},
					},
				},
			})
		}

		// Faire un autre appel pour obtenir la réponse aux résultats d'outils
		return h.processConversation(ctx, messages)
	}

	return nil
}

// RunServerMode démarre le serveur HTTP
func RunServerMode(ctx context.Context, provider llm.Provider, tools []llm.Tool, port int, messageWindowSize int) error {
	r := mux.NewRouter()

	handler := NewServerHandler(provider, tools, messageWindowSize)
	handler.Setup(ctx, r)

	// Ajouter un middleware de logging
	r.Use(func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			start := time.Now()
			next.ServeHTTP(w, r)
			log.Info("HTTP request",
				"method", r.Method,
				"path", r.URL.Path,
				"duration", time.Since(start),
				"remote_addr", r.RemoteAddr)
		})
	})

	// Démarrer le serveur
	server := &http.Server{
		Addr:         fmt.Sprintf(":%d", port),
		Handler:      r,
		ReadTimeout:  30 * time.Second,
		WriteTimeout: 90 * time.Second,
		IdleTimeout:  120 * time.Second,
	}

	// Canal pour gérer l'arrêt du serveur
	serverErrors := make(chan error, 1)

	go func() {
		log.Info("Démarrage du serveur HTTP", "port", port)
		serverErrors <- server.ListenAndServe()
	}()

	// Attendre l'arrêt ou une erreur
	select {
	case err := <-serverErrors:
		return fmt.Errorf("erreur du serveur: %v", err)
	case <-ctx.Done():
		log.Info("Arrêt du serveur en cours...")

		// Créer un contexte avec timeout pour l'arrêt
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()

		if err := server.Shutdown(shutdownCtx); err != nil {
			return fmt.Errorf("erreur lors de l'arrêt du serveur: %v", err)
		}

		log.Info("Serveur arrêté avec succès")
		return nil
	}
}
