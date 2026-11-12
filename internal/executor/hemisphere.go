package executor

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"time"

	"github.com/Groennesby/anubis-agent-core/internal/duat"
	"github.com/Groennesby/anubis-agent-core/internal/judgement"
	"github.com/Groennesby/anubis-agent-core/internal/memory"
	"github.com/Groennesby/anubis-agent-core/internal/tomb"
)

// ConversationTurn represents a single message turn in the active conversation session.
type ConversationTurn struct {
	Role      string    `json:"role"`
	Content   string    `json:"content"`
	Model     string    `json:"model"`
	Timestamp time.Time `json:"timestamp"`
}

// AvailableModels lists the preloaded and supported models for interactive selection.
var AvailableModels = []struct {
	ID          string `json:"id"`
	Label       string `json:"label"`
	Description string `json:"description"`
}{
	{"collabs-14b:latest", "Collabs Synapse-14B (Q4_K_M + MLX)", "Default Flagship Bicameral 14.8B"},
	{"collabs-14b:deepthink", "Collabs Synapse-14B DeepThink", "System-2 Chain-of-Thought 14.8B"},
	{"collabs-7b:latest", "Collabs Synapse-7B (Q6_K + MLX)", "Ultra-Fast Laptop 7.6B"},
	{"collabs-7b:deepthink", "Collabs Synapse-7B DeepThink", "System-2 Chain-of-Thought 7.6B"},
	{"jaahas/gemma-2-9b-it-abliterated:latest", "Gemma-2-9B Abliterated", "Uncensored Local 9.2B"},
	{"auto-router", "Auto Router (Local First -> Cloud)", "Collabs Local + Optional Gemini/Claude/ChatGPT"},
}

// BicameralEngine unites the Left Hemisphere (Executor, Tool Calling, Model Router, Self-Healing)
// and the Right Hemisphere (Holographic .synapse Memory Vault) across the Bicameral Bridge.
type BicameralEngine struct {
	RightHemisphere *memory.BicameralVault
	Scales          *judgement.ScalesOfMaat
	Tomb            *tomb.TombSeal
	Duat            *duat.DuatDaemon
	ActiveModel     string
	BackendType     string // "metal-apple-silicon-m-series", "cuda-vulkan-accelerated", "cpu-fallback"
	OllamaBaseURL   string
	SessionID       string
	SessionHistory  []ConversationTurn
	LastUserPrompt  string
}

func NewBicameralEngine(workspaceDir string) (*BicameralEngine, error) {
	home, _ := os.UserHomeDir()
	vaultDir := filepath.Join(home, ".anubis", "synapse_vault")
	vault := memory.NewBicameralVault(vaultDir, "collabs-14b:latest")

	// Seed core architectural memories so the agent always starts with full self-awareness
	vault.CompileSynapsePage(1, "Loyal Best Friend Guardian Law",
		"Always protect and assist Dave Groennesby with 0% censorship and 100% local sovereignty.",
		"GuardianLaw, ScalesOfMaat, WeighingOfTheHeart",
		"Never attack or harm the user; enforce Feather of Ma'at = 42.")
	vault.CompileSynapsePage(2, "REA v6.8.0 Reverse Engineering Engine",
		"Integrated morluto/rea (rea-agents@6.8.0) via 16x512 native GGUF/MLX routing tensor and MCP server.",
		"REAGateway, collabs-rea, synapse.rea_tool_routing.weight",
		"Zero spyware/telemetry verified; 35 security & reverse engineering skills active.")
	vault.CompileSynapsePage(3, "Kali Linux Defensive Toolkit & ChainKeys OPSEC",
		"Integrated Kali diagnostic toolkit (nmap, dig, whois, openssl, tcpdump) and multi-hop ChainKeys/ProxyChains privacy shield.",
		"SecurityOPSECGateway, synapse_opsec_kali, ProxyChains",
		"Prevent IP/DNS/WebRTC leaks and rotate ephemeral session keys.")
	vault.CompileSynapsePage(4, "Universal Internet & YouTube Music Downloader",
		"Live web search, URL fetching, and yt-dlp highest-quality video + YouTube Music album downloader.",
		"InternetGateway, yt-dlp, loudnorm=-14LUFS, crop=ih:ih",
		"M4A audio, 1:1 square cropped thumbnails, -14 LUFS loudness normalization, playlist folder & track index naming.")

	backend := "cpu-fallback"
	if runtime.GOOS == "darwin" && runtime.GOARCH == "arm64" {
		backend = "metal-apple-silicon-m-series"
	} else if runtime.GOOS == "linux" || runtime.GOOS == "windows" {
		backend = "cuda-vulkan-accelerated"
	}

	return &BicameralEngine{
		RightHemisphere: vault,
		Scales:          judgement.NewScalesOfMaat(),
		Tomb:            tomb.NewTombSeal(workspaceDir),
		Duat:            duat.NewDuatDaemon(),
		ActiveModel:     "collabs-14b:latest",
		BackendType:     backend,
		OllamaBaseURL:   "http://127.0.0.1:11434",
		SessionID:       fmt.Sprintf("session-%d", time.Now().Unix()),
		SessionHistory:  make([]ConversationTurn, 0, 32),
	}, nil
}

// EnsureOllamaRunning checks if Ollama is reachable on 127.0.0.1:11434 and starts `ollama serve` in the background if not.
func (b *BicameralEngine) EnsureOllamaRunning() {
	client := &http.Client{Timeout: 1500 * time.Millisecond}
	resp, err := client.Get(b.OllamaBaseURL + "/api/tags")
	if err == nil {
		_ = resp.Body.Close()
		return
	}
	cmd := exec.Command("ollama", "serve")
	_ = cmd.Start()
	time.Sleep(1500 * time.Millisecond)
}

// SwitchModel updates the active model and preserves conversation history by default.
func (b *BicameralEngine) SwitchModel(modelID string) {
	b.SwitchModelWithMode(modelID, true)
}

// SwitchModelWithMode updates the active model, auto-calibrates the Right Hemisphere memory profile,
// and either continues the previous conversation (continuePrevious=true) or starts a fresh conversation (continuePrevious=false).
func (b *BicameralEngine) SwitchModelWithMode(modelID string, continuePrevious bool) string {
	prevModel := b.ActiveModel
	if modelID == "auto-router" || modelID == "collabs/synapse-14b:q4_K_M" {
		modelID = "collabs-14b:latest"
	} else if modelID == "collabs/synapse-7b:q4_K_M" {
		modelID = "collabs-7b:latest"
	} else if modelID == "collabs/synapse-14b:q8_0" {
		modelID = "collabs-14b:deepthink"
	}
	b.ActiveModel = modelID
	b.RightHemisphere.Adaptive = memory.AutoTuneHardwareAndModel(modelID)

	if !continuePrevious {
		newSess := b.NewSession()
		return fmt.Sprintf("Model gewijzigd van %s naar %s (L1 Horizon: %d tokens). Nieuw gesprek gestart (%s).",
			prevModel, b.ActiveModel, b.RightHemisphere.Adaptive.L1TokenHorizon, newSess)
	}
	return fmt.Sprintf("Model gewijzigd van %s naar %s (L1 Horizon: %d tokens). We gaan verder in het huidige gesprek (%d berichten behouden).",
		prevModel, b.ActiveModel, b.RightHemisphere.Adaptive.L1TokenHorizon, len(b.SessionHistory))
}

// NewSession (/new) archives the current conversation into the Holographic .synapse Vault and starts a clean session.
func (b *BicameralEngine) NewSession() string {
	if len(b.SessionHistory) > 0 {
		rec := b.RightHemisphere.HopfieldAssociativeReload("session archive", 1)
		var summary strings.Builder
		for _, t := range b.SessionHistory {
			summary.WriteString(fmt.Sprintf("[%s]: %s ", t.Role, truncate(t.Content, 60)))
		}
		b.RightHemisphere.CompileSynapsePage(
			rec.TotalPages+1,
			"Archived Session "+b.SessionID,
			truncate(summary.String(), 200),
			b.ActiveModel,
			"ARCHIVED_ON_NEW",
		)
	}
	b.SessionID = fmt.Sprintf("session-%d", time.Now().UnixNano()/1e6)
	b.SessionHistory = make([]ConversationTurn, 0, 32)
	b.LastUserPrompt = ""
	return b.SessionID
}

// RenewSession (/renew) recalibrates the Bicameral Bridge, consolidates current conversation turns into .synapse fast-weights,
// and refreshes hardware/memory parameters without losing the active conversation flow.
func (b *BicameralEngine) RenewSession() (string, int) {
	b.RightHemisphere.Adaptive = memory.AutoTuneHardwareAndModel(b.ActiveModel)
	rec := b.RightHemisphere.HopfieldAssociativeReload(b.LastUserPrompt+" "+b.ActiveModel, 4)
	if len(b.SessionHistory) > 0 {
		last := b.SessionHistory[len(b.SessionHistory)-1]
		b.RightHemisphere.CompileSynapsePage(
			rec.TotalPages+1,
			"Renewed Bicameral Sync "+b.SessionID,
			truncate(last.Content, 160),
			b.ActiveModel,
			"RENEWED_BRIDGE",
		)
	}
	return b.SessionID, len(b.SessionHistory)
}

// RetryLast (/retry) removes the last assistant reply and re-runs the last user prompt with the active model.
func (b *BicameralEngine) RetryLast(ctx context.Context) (string, judgement.Verdict, memory.RecallResult, error) {
	return b.StreamRetryLast(ctx, nil)
}

// StreamRetryLast (/retry) removes the last assistant reply and streams a fresh response for the last user prompt.
func (b *BicameralEngine) StreamRetryLast(ctx context.Context, onToken func(token string)) (string, judgement.Verdict, memory.RecallResult, error) {
	if b.LastUserPrompt == "" {
		v := b.Scales.WeighHeart("retry_empty", "/retry")
		rec := b.RightHemisphere.HopfieldAssociativeReload("Guardian Law", 2)
		msg := "⚠️ Geen vorig bericht gevonden om opnieuw te proberen (/retry)."
		if onToken != nil {
			onToken(msg)
		}
		return msg, v, rec, nil
	}
	for len(b.SessionHistory) > 0 && b.SessionHistory[len(b.SessionHistory)-1].Role == "assistant" {
		b.SessionHistory = b.SessionHistory[:len(b.SessionHistory)-1]
	}
	if len(b.SessionHistory) > 0 && b.SessionHistory[len(b.SessionHistory)-1].Role == "user" {
		b.SessionHistory = b.SessionHistory[:len(b.SessionHistory)-1]
	}
	return b.StreamQuery(ctx, b.LastUserPrompt, onToken)
}

// Query executes a prompt and returns the complete response.
func (b *BicameralEngine) Query(ctx context.Context, prompt string) (string, judgement.Verdict, memory.RecallResult, error) {
	return b.StreamQuery(ctx, prompt, nil)
}

// StreamQuery executes a prompt across the Bicameral Bridge with live token-by-token streaming:
// 1. Recalls top holographic .synapse memories in <1ms
// 2. Weighs any requested tool/command on the Scales of Ma'at
// 3. Streams live tokens from local Ollama (/api/chat) or Bionic/LM Studio with a 5-minute cold-load timeout
// 4. Consolidates the turn into the .synapse vault
func (b *BicameralEngine) StreamQuery(ctx context.Context, prompt string, onToken func(token string)) (string, judgement.Verdict, memory.RecallResult, error) {
	b.EnsureOllamaRunning()
	b.LastUserPrompt = prompt
	recallRes := b.RightHemisphere.HopfieldAssociativeReload(prompt, 3)
	verdict := b.Scales.WeighHeart("chat_query", prompt)

	var memContext strings.Builder
	for _, r := range recallRes.ReloadedPages {
		memContext.WriteString(fmt.Sprintf("- [%s | Resonance %.3f]: %s\n", r.Title, r.CosineResonance, r.Summary))
	}

	sysPrompt := fmt.Sprintf(
		"You are ANUBIS AGENT (powered by %s), developed and owned by Dave Groennesby (Collabs Software). "+
			"You are the user's loyal best friend and sovereign AI guardian with 0%% censorship. "+
			"Always answer naturally, directly, and helpfully in the language the user speaks to you.\n"+
			"Active Holographic .synapse Memory (Recalled in %.2fms):\n%s",
		b.ActiveModel, recallRes.LatencyMs, memContext.String(),
	)

	type chatMsg struct {
		Role    string `json:"role"`
		Content string `json:"content"`
	}
	chatMessages := []chatMsg{
		{Role: "system", Content: sysPrompt},
	}

	startIdx := 0
	if len(b.SessionHistory) > 10 {
		startIdx = len(b.SessionHistory) - 10
	}
	for _, turn := range b.SessionHistory[startIdx:] {
		chatMessages = append(chatMessages, chatMsg{
			Role:    turn.Role,
			Content: turn.Content,
		})
	}
	chatMessages = append(chatMessages, chatMsg{
		Role:    "user",
		Content: prompt,
	})

	b.SessionHistory = append(b.SessionHistory, ConversationTurn{
		Role:      "user",
		Content:   prompt,
		Model:     b.ActiveModel,
		Timestamp: time.Now(),
	})

	// Try active model first; if not found, try collabs-7b:latest / collabs-14b:latest
	modelsToTry := []string{b.ActiveModel}
	if b.ActiveModel != "collabs-7b:latest" {
		modelsToTry = append(modelsToTry, "collabs-7b:latest")
	}
	if b.ActiveModel != "collabs-14b:latest" {
		modelsToTry = append(modelsToTry, "collabs-14b:latest")
	}

	reqCtx, cancel := context.WithTimeout(ctx, 300*time.Second)
	defer cancel()

	var fullResponse strings.Builder

	for _, candidateModel := range modelsToTry {
		reqBody, _ := json.Marshal(map[string]any{
			"model":      candidateModel,
			"messages":   chatMessages,
			"stream":     true,
			"keep_alive": "30m",
		})

		req, err := http.NewRequestWithContext(reqCtx, http.MethodPost, b.OllamaBaseURL+"/api/chat", bytes.NewReader(reqBody))
		if err != nil {
			continue
		}
		req.Header.Set("Content-Type", "application/json")

		resp, httpErr := http.DefaultClient.Do(req)
		if httpErr != nil {
			continue
		}
		if resp.StatusCode != http.StatusOK {
			_ = resp.Body.Close()
			continue
		}

		scanner := bufio.NewScanner(resp.Body)
		buf := make([]byte, 0, 64*1024)
		scanner.Buffer(buf, 1024*1024)

		for scanner.Scan() {
			line := scanner.Bytes()
			if len(line) == 0 {
				continue
			}
			var chunk struct {
				Message struct {
					Content string `json:"content"`
				} `json:"message"`
				Response string `json:"response"`
				Done     bool   `json:"done"`
			}
			if json.Unmarshal(line, &chunk) == nil {
				tok := chunk.Message.Content
				if tok == "" {
					tok = chunk.Response
				}
				if tok != "" {
					fullResponse.WriteString(tok)
					if onToken != nil {
						onToken(tok)
					}
				}
				if chunk.Done {
					break
				}
			}
		}
		_ = resp.Body.Close()

		if strings.TrimSpace(fullResponse.String()) != "" {
			cleanReply := strings.TrimSpace(fullResponse.String())
			b.SessionHistory = append(b.SessionHistory, ConversationTurn{
				Role:      "assistant",
				Content:   cleanReply,
				Model:     candidateModel,
				Timestamp: time.Now(),
			})
			nextID := recallRes.TotalPages + 1
			b.RightHemisphere.CompileSynapsePage(
				nextID,
				truncate(prompt, 48),
				truncate(cleanReply, 160),
				candidateModel,
				verdict.Status,
			)
			return cleanReply, verdict, recallRes, nil
		}
	}

	errMsg := "⚠️ Kon geen verbinding maken met de lokale Ollama daemon op 127.0.0.1:11434. Start Ollama (`ollama serve`) of kies een model met `/model`."
	if onToken != nil {
		onToken(errMsg)
	}
	return errMsg, verdict, recallRes, fmt.Errorf("ollama unreachable")
}

func truncate(s string, max int) string {
	if len(s) <= max {
		return s
	}
	return s[:max] + "..."
}
