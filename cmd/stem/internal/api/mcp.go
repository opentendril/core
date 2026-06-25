package api

import (
	"encoding/json"
	"log"
	"net/http"
	"strings"

	"github.com/opentendril/core/cmd/stem/internal/orchestrator"
)

type MCPHandler struct{}

func NewMCPHandler() *MCPHandler {
	return &MCPHandler{}
}

func (h *MCPHandler) SetupRoutes(mux *http.ServeMux) {
	mux.HandleFunc("/v1", h.HandleMCP)
}

type mcpRequest struct {
	JSONRPC string          `json:"jsonrpc"`
	ID      interface{}     `json:"id"`
	Method  string          `json:"method"`
	Params  json.RawMessage `json:"params,omitempty"`
}

type mcpResponse struct {
	JSONRPC string      `json:"jsonrpc"`
	ID      interface{} `json:"id"`
	Result  interface{} `json:"result,omitempty"`
	Error   *mcpError   `json:"error,omitempty"`
}

type mcpError struct {
	Code    int         `json:"code"`
	Message string      `json:"message"`
	Data    interface{} `json:"data,omitempty"`
}

func (h *MCPHandler) HandleMCP(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var req mcpRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		h.sendError(w, nil, -32700, "Parse error", err.Error())
		return
	}

	switch req.Method {
	case "tools/list":
		h.sendResult(w, req.ID, map[string]interface{}{
			"tools": []map[string]interface{}{
				{
					"name":        "tendrilExecute",
					"description": "Delegates a complex coding task to the autonomous OpenTendril brain. Use this tool when you need an agent to run terminal commands, debug complex errors, search the web, or execute multi-step engineering tasks inside a secure sandbox.",
					"inputSchema": map[string]interface{}{
						"type": "object",
						"properties": map[string]interface{}{
							"task": map[string]interface{}{
								"type":        "string",
								"description": "A clear, actionable description of the task for Tendril to execute.",
							},
						},
						"required": []string{"task"},
					},
				},
			},
		})

	case "tools/call":
		var params struct {
			Name      string                 `json:"name"`
			Arguments map[string]interface{} `json:"arguments"`
		}
		if err := json.Unmarshal(req.Params, &params); err != nil {
			h.sendError(w, req.ID, -32602, "Invalid params", err.Error())
			return
		}

		if params.Name != "tendrilExecute" {
			h.sendError(w, req.ID, -32601, "Tool not found", nil)
			return
		}

		task, ok := params.Arguments["task"].(string)
		if !ok || strings.TrimSpace(task) == "" {
			h.sendError(w, req.ID, -32602, "Invalid arguments", "The 'task' parameter is required.")
			return
		}

		log.Printf("[MCP] Delegating task to Tendril: %s", task)
		orch := &orchestrator.DockerOrchestrator{
			ImageName: "opentendril-tendril:latest",
		}
		output, err := orch.RunTendril(r.Context(), task)
		if err != nil {
			log.Printf("[MCP] Tendril execution failed: %v", err)
			h.sendResult(w, req.ID, map[string]interface{}{
				"content": []map[string]interface{}{
					{
						"type": "text",
						"text": "Task execution failed: " + err.Error(),
					},
				},
				"isError": true,
			})
			return
		}

		h.sendResult(w, req.ID, map[string]interface{}{
			"content": []map[string]interface{}{
				{
					"type": "text",
					"text": output,
				},
			},
			"isError": false,
		})

	default:
		h.sendError(w, req.ID, -32601, "Method not found", nil)
	}
}

func (h *MCPHandler) sendResult(w http.ResponseWriter, id interface{}, result interface{}) {
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(mcpResponse{
		JSONRPC: "2.0",
		ID:      id,
		Result:  result,
	})
}

func (h *MCPHandler) sendError(w http.ResponseWriter, id interface{}, code int, msg string, data interface{}) {
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(mcpResponse{
		JSONRPC: "2.0",
		ID:      id,
		Error: &mcpError{
			Code:    code,
			Message: msg,
			Data:    data,
		},
	})
}
