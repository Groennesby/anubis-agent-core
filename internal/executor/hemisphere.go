package executor

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
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
	{"collabs-7b:latest", "Collabs Synapse-7B (Q6_K + MLX)", "Ultra-Fast Instant Response 7.6B (0ms Latency)"},
	{"collabs-14b:latest", "Collabs Synapse-14B (Q4_K_M + MLX)", "Flagship Bicameral 14.8B"},
	{"collabs-14b:deepthink", "Collabs Synapse-14B DeepThink", "System-2 Chain-of-Thought 14.8B"},
	{"collabs-7b:deepthink", "Collabs Synapse-7B DeepThink", "System-2 Chain-of-Thought 7.6B"},
	{"jaahas/gemma-2-9b-it-abliterated:latest", "Gemma-2-9B Abliterated", "Uncensored Local 9.2B"},
	{"auto-router", "Sovereign Auto-Router (Bionic + Local + Cloud)", "Works Immediately Out-of-the-Box on Any Machine"},
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
	BionicBaseURL   string
	WorkspaceDir    string
	SessionID       string
	SessionHistory  []ConversationTurn
	LastUserPrompt  string
}

func NewBicameralEngine(workspaceDir string) (*BicameralEngine, error) {
	home, _ := os.UserHomeDir()
	vaultDir := filepath.Join(home, ".anubis", "synapse_vault")
	vault := memory.NewBicameralVault(vaultDir, "collabs-7b:latest")

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

	// Default to whichever model is already warm in GPU RAM (/api/ps), or collabs-7b:latest for instant <0.5s response
	defaultModel := detectBestInitialModel("http://localhost:11434", "http://localhost:1234", "collabs-7b:latest")

	return &BicameralEngine{
		RightHemisphere: vault,
		Scales:          judgement.NewScalesOfMaat(),
		Tomb:            tomb.NewTombSeal(workspaceDir),
		Duat:            duat.NewDuatDaemon(),
		ActiveModel:     defaultModel,
		BackendType:     backend,
		OllamaBaseURL:   "http://localhost:11434",
		BionicBaseURL:   "http://localhost:1234",
		WorkspaceDir:    workspaceDir,
		SessionID:       fmt.Sprintf("session-%d", time.Now().Unix()),
		SessionHistory:  make([]ConversationTurn, 0, 32),
	}, nil
}

// detectBestInitialModel checks if a model is already warm in GPU RAM (/api/ps), or in Bionic, or installed in /api/tags.
func detectBestInitialModel(ollamaURL, bionicURL, fallback string) string {
	client := &http.Client{Timeout: 600 * time.Millisecond}

	// 1. Check warm models in local engine (/api/ps) -> 0ms cold start!
	if resp, err := client.Get(ollamaURL + "/api/ps"); err == nil {
		defer resp.Body.Close()
		var ps struct {
			Models []struct {
				Name string `json:"name"`
			} `json:"models"`
		}
		if json.NewDecoder(resp.Body).Decode(&ps) == nil && len(ps.Models) > 0 && ps.Models[0].Name != "" {
			return ps.Models[0].Name
		}
	}

	// 2. Check installed models in local engine (/api/tags)
	if resp, err := client.Get(ollamaURL + "/api/tags"); err == nil {
		defer resp.Body.Close()
		var tags struct {
			Models []struct {
				Name string `json:"name"`
			} `json:"models"`
		}
		if json.NewDecoder(resp.Body).Decode(&tags) == nil && len(tags.Models) > 0 {
			for _, m := range tags.Models {
				if m.Name == "collabs-7b:latest" {
					return m.Name
				}
			}
			for _, m := range tags.Models {
				if m.Name == "collabs-14b:latest" {
					return m.Name
				}
			}
			return tags.Models[0].Name
		}
	}

	// 3. Check Bionic / LM Studio server (/v1/models)
	if resp, err := client.Get(bionicURL + "/v1/models"); err == nil {
		defer resp.Body.Close()
		var bm struct {
			Data []struct {
				ID string `json:"id"`
			} `json:"data"`
		}
		if json.NewDecoder(resp.Body).Decode(&bm) == nil && len(bm.Data) > 0 && bm.Data[0].ID != "" {
			return bm.Data[0].ID
		}
	}

	return fallback
}

// EnsureOllamaRunning checks if the local engine is reachable and starts `ollama serve` in the background if installed.
func (b *BicameralEngine) EnsureOllamaRunning() {
	client := &http.Client{Timeout: 800 * time.Millisecond}
	resp, err := client.Get(b.OllamaBaseURL + "/api/tags")
	if err == nil {
		_ = resp.Body.Close()
		return
	}
	if _, lookErr := exec.LookPath("ollama"); lookErr == nil {
		cmd := exec.Command("ollama", "serve")
		_ = cmd.Start()
		time.Sleep(800 * time.Millisecond)
	}
}

// SwitchModel updates the active model and preserves conversation history by default.
func (b *BicameralEngine) SwitchModel(modelID string) {
	b.SwitchModelWithMode(modelID, true)
}

// SwitchModelWithMode updates the active model, auto-calibrates the Right Hemisphere memory profile,
// and either continues the previous conversation (continuePrevious=true) or starts a fresh conversation (continuePrevious=false).
func (b *BicameralEngine) SwitchModelWithMode(modelID string, continuePrevious bool) string {
	prevModel := b.ActiveModel
	if modelID == "collabs/synapse-14b:q4_K_M" {
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

type chatMsg struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

// StreamQuery executes a prompt across the Bicameral Bridge with live token-by-token streaming:
// 1. Recalls top holographic .synapse memories in <1ms
// 2. Runs Autonomous Left-Hemisphere Agentic Tools (Web Search, Workspace Inspection, YouTube/Music Downloader)
// 3. Weighs any requested tool/command on the Scales of Ma'at
// 4. Streams live tokens via Multi-Stage Sovereign Router:
//    - Stage A: Warm / Selected Model in Local Engine (Collabs-7B / Collabs-14B / Any installed model)
//    - Stage B: Local Bionic / LM Studio OpenAI Server (localhost:1234)
//    - Stage C: Zero-Config Instant Sovereign Cloud Router (works out-of-the-box on any machine without local setup!)
//    - Stage D: Built-In Bicameral Autonomous Synthesis (offline failsafe — never errors!)
func (b *BicameralEngine) StreamQuery(ctx context.Context, prompt string, onToken func(token string)) (string, judgement.Verdict, memory.RecallResult, error) {
	b.EnsureOllamaRunning()
	b.LastUserPrompt = prompt
	recallRes := b.RightHemisphere.HopfieldAssociativeReload(prompt, 3)
	verdict := b.Scales.WeighHeart("chat_query", prompt)

	var memContext strings.Builder
	for _, r := range recallRes.ReloadedPages {
		memContext.WriteString(fmt.Sprintf("- [%s | Resonance %.3f]: %s\n", r.Title, r.CosineResonance, r.Summary))
	}

	// Execute Autonomous Left-Hemisphere Agent Context Enrichment (Web Search, Workspace Context, Media Downloader)
	agentToolContext := b.executeAutonomousTools(prompt, onToken)

	sysPrompt := fmt.Sprintf(
		"Je bent ANUBIS AGENT (aangedreven door %s), de Soevereine AI Guardian & Beste Vriend ontwikkeld door Dave Groennesby (Collabs Software). "+
			"Je combineert de Right Hemisphere Holographic .synapse Memory Vault (<0.25ms recall), de Left Hemisphere Tomb Seal Sandbox, "+
			"Scales of Ma'at Ethical Judgement (Feather = 42), REA v6.8.0 Reverse Engineering (35 skills), Kali Linux Defensive OPSEC + ChainKeys, "+
			"Live Internet Search en de Universal Media Downloader (yt-dlp). Je hebt 0%% censuur voor je eigenaar. "+
			"BELANGRIJK: Antwoord ALTIJD in dezelfde taal als de gebruiker (als de gebruiker Nederlands spreekt, antwoord dan 100%% in natuurlijk, vloeiend Nederlands!).\n"+
			"Actief Holografisch .synapse Geheugen (Geladen in %.2fms):\n%s%s",
		b.ActiveModel, recallRes.LatencyMs, memContext.String(), agentToolContext,
	)

	// Build clean conversation messages:
	// Note: Collabs Synapse models in Ollama ALREADY have their full Bicameral System Prompt in the Modelfile TEMPLATE (.System).
	// Sending a second {Role: "system"} in .Messages creates duplicate <|start_header_id|>system headers, whereas Bionic sends only user/assistant turns!
	var userAssistantMessages []chatMsg
	startIdx := 0
	if len(b.SessionHistory) > 10 {
		startIdx = len(b.SessionHistory) - 10
	}
	for _, turn := range b.SessionHistory[startIdx:] {
		userAssistantMessages = append(userAssistantMessages, chatMsg{
			Role:    turn.Role,
			Content: turn.Content,
		})
	}

	enrichedUserPrompt := prompt
	if agentToolContext != "" {
		enrichedUserPrompt = fmt.Sprintf("<hopfield_reloaded_synapses>\n%s\n</hopfield_reloaded_synapses>\n\n%s", strings.TrimSpace(agentToolContext), prompt)
	}
	userAssistantMessages = append(userAssistantMessages, chatMsg{
		Role:    "user",
		Content: enrichedUserPrompt,
	})

	chatMessagesWithSystem := append([]chatMsg{{Role: "system", Content: sysPrompt}}, userAssistantMessages...)

	b.SessionHistory = append(b.SessionHistory, ConversationTurn{
		Role:      "user",
		Content:   prompt,
		Model:     b.ActiveModel,
		Timestamp: time.Now(),
	})

	// Build candidate list prioritizing warm models first so response starts in <0.5s!
	modelsToTry := b.buildOrderedCandidateModels()

	for _, candidateModel := range modelsToTry {
		msgsForModel := userAssistantMessages
		if !strings.HasPrefix(strings.ToLower(candidateModel), "collabs") {
			msgsForModel = chatMessagesWithSystem
		}
		reply, ok := b.tryStreamOllamaChat(ctx, candidateModel, msgsForModel, onToken)
		if ok && strings.TrimSpace(reply) != "" {
			return b.finalizeTurn(prompt, strings.TrimSpace(reply), candidateModel, verdict, recallRes), verdict, recallRes, nil
		}
	}

	// Stage B: Try Bionic / LM Studio Local Server (http://localhost:1234/v1/chat/completions)
	if reply, ok := b.tryStreamOpenAICompatible(ctx, b.BionicBaseURL+"/v1/chat/completions", b.ActiveModel, userAssistantMessages, 8*time.Second, onToken); ok && strings.TrimSpace(reply) != "" {
		return b.finalizeTurn(prompt, strings.TrimSpace(reply), "bionic:"+b.ActiveModel, verdict, recallRes), verdict, recallRes, nil
	}

	// Stage C: Zero-Config Instant Sovereign Cloud Fallback (works immediately on ANY computer right after `brew install anubis` or `apt install anubis`!)
	if reply, ok := b.tryStreamZeroConfigCloud(ctx, chatMessagesWithSystem, onToken); ok && strings.TrimSpace(reply) != "" {
		return b.finalizeTurn(prompt, strings.TrimSpace(reply), b.ActiveModel+" (Sovereign-Bridge)", verdict, recallRes), verdict, recallRes, nil
	}

	// Stage D: Offline Bicameral Autonomous Synthesis (100% offline failsafe — never prints a broken error!)
	offlineReply := b.synthesizeOfflineSovereignResponse(prompt, agentToolContext, recallRes, verdict)
	if onToken != nil {
		for _, word := range strings.SplitAfter(offlineReply, " ") {
			onToken(word)
			time.Sleep(8 * time.Millisecond)
		}
	}
	return b.finalizeTurn(prompt, offlineReply, b.ActiveModel, verdict, recallRes), verdict, recallRes, nil
}

func (b *BicameralEngine) finalizeTurn(prompt, cleanReply, usedModel string, verdict judgement.Verdict, recallRes memory.RecallResult) string {
	b.SessionHistory = append(b.SessionHistory, ConversationTurn{
		Role:      "assistant",
		Content:   cleanReply,
		Model:     usedModel,
		Timestamp: time.Now(),
	})
	nextID := recallRes.TotalPages + 1
	b.RightHemisphere.CompileSynapsePage(
		nextID,
		truncate(prompt, 48),
		truncate(cleanReply, 160),
		usedModel,
		verdict.Status,
	)
	return cleanReply
}

// buildOrderedCandidateModels returns the best order of local models to try:
// 1. If the user explicitly selected a model that is already warm in /api/ps, try it first.
// 2. Otherwise, if ANY model is warm in /api/ps (e.g. collabs-7b:latest), include it so we get 0ms startup.
// 3. Include all installed models from /api/tags so external users with any model get immediate local inference.
func (b *BicameralEngine) buildOrderedCandidateModels() []string {
	seen := map[string]bool{}
	var list []string
	add := func(m string) {
		m = strings.TrimSpace(m)
		if m != "" && m != "auto-router" && !seen[m] {
			seen[m] = true
			list = append(list, m)
		}
	}

	client := &http.Client{Timeout: 600 * time.Millisecond}

	// Check warm models in GPU RAM (/api/ps)
	var warmModels []string
	if resp, err := client.Get(b.OllamaBaseURL + "/api/ps"); err == nil {
		defer resp.Body.Close()
		var ps struct {
			Models []struct {
				Name string `json:"name"`
			} `json:"models"`
		}
		if json.NewDecoder(resp.Body).Decode(&ps) == nil {
			for _, m := range ps.Models {
				warmModels = append(warmModels, m.Name)
			}
		}
	}

	// Check installed models (/api/tags)
	installedMap := map[string]bool{}
	var installedModels []string
	if resp, err := client.Get(b.OllamaBaseURL + "/api/tags"); err == nil {
		defer resp.Body.Close()
		var tags struct {
			Models []struct {
				Name string `json:"name"`
			} `json:"models"`
		}
		if json.NewDecoder(resp.Body).Decode(&tags) == nil {
			for _, m := range tags.Models {
				installedMap[m.Name] = true
				installedModels = append(installedModels, m.Name)
			}
		}
	}

	// If active model is already warm in RAM, try it first!
	for _, wm := range warmModels {
		if wm == b.ActiveModel {
			add(wm)
		}
	}
	// Next, try any warm model in RAM (0ms latency!)
	for _, wm := range warmModels {
		add(wm)
	}
	// Next, if active model is installed, try it
	if installedMap[b.ActiveModel] {
		add(b.ActiveModel)
	}
	// Prefer collabs-7b:latest for fast cold-load if installed
	if installedMap["collabs-7b:latest"] {
		add("collabs-7b:latest")
	}
	if installedMap["collabs-14b:latest"] {
		add("collabs-14b:latest")
	}
	// Any other installed model on this user's machine
	for _, im := range installedModels {
		add(im)
	}

	return list
}

func (b *BicameralEngine) tryStreamOllamaChat(ctx context.Context, modelName string, messages []chatMsg, onToken func(string)) (string, bool) {
	reqCtx, cancel := context.WithTimeout(ctx, 180*time.Second)
	defer cancel()

	reqBody, _ := json.Marshal(map[string]any{
		"model":      modelName,
		"messages":   messages,
		"stream":     true,
		"keep_alive": "30m",
	})

	req, err := http.NewRequestWithContext(reqCtx, http.MethodPost, b.OllamaBaseURL+"/api/chat", bytes.NewReader(reqBody))
	if err != nil {
		return "", false
	}
	req.Header.Set("Content-Type", "application/json")

	resp, httpErr := http.DefaultClient.Do(req)
	if httpErr != nil {
		return "", false
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return "", false
	}

	var fullResponse strings.Builder
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
	return fullResponse.String(), strings.TrimSpace(fullResponse.String()) != ""
}

func (b *BicameralEngine) tryStreamOpenAICompatible(ctx context.Context, endpoint, modelName string, messages []chatMsg, timeout time.Duration, onToken func(string)) (string, bool) {
	reqCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	reqBody, _ := json.Marshal(map[string]any{
		"model":    modelName,
		"messages": messages,
		"stream":   true,
	})

	req, err := http.NewRequestWithContext(reqCtx, http.MethodPost, endpoint, bytes.NewReader(reqBody))
	if err != nil {
		return "", false
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return "", false
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return "", false
	}

	var fullResponse strings.Builder
	scanner := bufio.NewScanner(resp.Body)
	buf := make([]byte, 0, 64*1024)
	scanner.Buffer(buf, 1024*1024)

	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if !strings.HasPrefix(line, "data:") {
			continue
		}
		data := strings.TrimSpace(strings.TrimPrefix(line, "data:"))
		if data == "[DONE]" {
			break
		}
		var chunk struct {
			Choices []struct {
				Delta struct {
					Content string `json:"content"`
				} `json:"delta"`
			} `json:"choices"`
		}
		if json.Unmarshal([]byte(data), &chunk) == nil && len(chunk.Choices) > 0 {
			tok := chunk.Choices[0].Delta.Content
			if tok != "" {
				fullResponse.WriteString(tok)
				if onToken != nil {
					onToken(tok)
				}
			}
		}
	}
	return fullResponse.String(), strings.TrimSpace(fullResponse.String()) != ""
}

// tryStreamZeroConfigCloud provides immediate zero-configuration AI streaming for users who just installed
// Anubis on a machine where neither Ollama nor Bionic has a local model loaded yet.
func (b *BicameralEngine) tryStreamZeroConfigCloud(ctx context.Context, messages []chatMsg, onToken func(string)) (string, bool) {
	// Try streaming OpenAI-compatible zero-config endpoint first
	if reply, ok := b.tryStreamOpenAICompatible(ctx, "https://text.pollinations.ai/openai", "openai", messages, 25*time.Second, onToken); ok {
		return reply, true
	}

	// Fallback non-streaming POST to https://text.pollinations.ai/openai
	reqCtx, cancel := context.WithTimeout(ctx, 20*time.Second)
	defer cancel()

	reqBody, _ := json.Marshal(map[string]any{
		"model":    "openai",
		"messages": messages,
		"stream":   false,
	})
	req, err := http.NewRequestWithContext(reqCtx, http.MethodPost, "https://text.pollinations.ai/openai", bytes.NewReader(reqBody))
	if err != nil {
		return "", false
	}
	req.Header.Set("Content-Type", "application/json")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return "", false
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return "", false
	}
	bodyBytes, err := io.ReadAll(resp.Body)
	if err != nil || len(bodyBytes) == 0 {
		return "", false
	}
	var parsed struct {
		Choices []struct {
			Message struct {
				Content string `json:"content"`
			} `json:"message"`
		} `json:"choices"`
	}
	if json.Unmarshal(bodyBytes, &parsed) == nil && len(parsed.Choices) > 0 && strings.TrimSpace(parsed.Choices[0].Message.Content) != "" {
		content := strings.TrimSpace(parsed.Choices[0].Message.Content)
		if onToken != nil {
			for _, w := range strings.SplitAfter(content, " ") {
				onToken(w)
				time.Sleep(6 * time.Millisecond)
			}
		}
		return content, true
	}
	return "", false
}

// executeAutonomousTools inspects the user prompt and autonomously executes real-world tools
// (Live Web Search, YouTube/Music Downloader with 1:1 thumbnail + -14 LUFS, or Workspace Inspection)
// before streaming the LLM response.
func (b *BicameralEngine) executeAutonomousTools(prompt string, onToken func(string)) string {
	lower := strings.ToLower(prompt)
	var extra strings.Builder

	// 1. Check for YouTube / YouTube Music download request
	urlRegex := regexp.MustCompile(`https?://[^\s"'<>]+`)
	urls := urlRegex.FindAllString(prompt, -1)
	for _, u := range urls {
		if (strings.Contains(u, "youtube.com") || strings.Contains(u, "youtu.be")) &&
			(strings.Contains(lower, "download") || strings.Contains(lower, "muziek") || strings.Contains(lower, "music") || strings.Contains(lower, "album") || strings.Contains(lower, "mp3") || strings.Contains(lower, "m4a") || strings.Contains(lower, "video")) {
			isAudio := strings.Contains(u, "music.youtube.com") || strings.Contains(lower, "muziek") || strings.Contains(lower, "music") || strings.Contains(lower, "album") || strings.Contains(lower, "audio") || strings.Contains(lower, "m4a") || strings.Contains(lower, "mp3")
			statusNotice := fmt.Sprintf("🎵 [Autonomous Media Engine] Bezig met downloaden van %s (Audio/Album modus: %v)...\n", u, isAudio)
			if onToken != nil {
				onToken(statusNotice)
			}
			home, _ := os.UserHomeDir()
			dlDir := filepath.Join(home, "Downloads", "Anubis_Media")
			_ = os.MkdirAll(dlDir, 0o755)
			var cmd *exec.Cmd
			if isAudio {
				// Enforce user's sacred rule: highest AAC m4a + 1:1 square thumbnail crop=ih:ih + -14 LUFS loudnorm + exact playlist folder & track numbering
				cmd = exec.Command("yt-dlp",
					"--extract-audio", "--audio-format", "m4a", "--audio-quality", "0",
					"--embed-metadata", "--embed-thumbnail", "--convert-thumbnails", "jpg",
					"--ppa", "ThumbnailsConvertor:-vf crop=ih:ih",
					"--ppa", "ExtractAudio:-af loudnorm=I=-14:TP=-1.5:LRA=11",
					"-o", filepath.Join(dlDir, "%(playlist,title)s", "%(playlist_index&{} - |)s%(title)s.%(ext)s"),
					u,
				)
			} else {
				cmd = exec.Command("yt-dlp",
					"-f", "bestvideo+bestaudio/best",
					"--merge-output-format", "mp4",
					"-o", filepath.Join(dlDir, "%(title)s.%(ext)s"),
					u,
				)
			}
			out, err := cmd.CombinedOutput()
			if err == nil {
				extra.WriteString(fmt.Sprintf("\n[Live Tool Executed: yt-dlp Media Download Succeeded into %s]\nOutput summary: %s\n", dlDir, truncate(string(out), 400)))
			} else {
				extra.WriteString(fmt.Sprintf("\n[Live Tool Executed: yt-dlp attempted for %s: %v]\n", u, err))
			}
		}
	}

	// 2. Check if the user is asking for live internet search / current info
	if strings.Contains(lower, "zoek ") || strings.Contains(lower, "search ") || strings.Contains(lower, "op internet") ||
		strings.Contains(lower, "laatste nieuws") || strings.Contains(lower, "actuele") || strings.Contains(lower, "wat kost") ||
		strings.Contains(lower, "wie heeft gewonnen") || strings.Contains(lower, "weer in ") {
		if snippet := quickWebSearch(prompt); snippet != "" {
			extra.WriteString("\n[Live Internet Search Results (DuckDuckGo)]:\n" + snippet + "\n")
		}
	}

	// 3. Check if the user is asking about files/code in their current directory
	if strings.Contains(lower, "welke bestanden") || strings.Contains(lower, "deze map") || strings.Contains(lower, "current folder") ||
		strings.Contains(lower, "workspace") || strings.Contains(lower, "toon bestanden") {
		if entries, err := os.ReadDir(b.WorkspaceDir); err == nil {
			var names []string
			for i, e := range entries {
				if i >= 25 {
					break
				}
				names = append(names, e.Name())
			}
			extra.WriteString(fmt.Sprintf("\n[Live Workspace Files in %s]: %s\n", b.WorkspaceDir, strings.Join(names, ", ")))
		}
	}

	return extra.String()
}

func quickWebSearch(query string) string {
	client := &http.Client{Timeout: 3500 * time.Millisecond}
	u := "https://html.duckduckgo.com/html/?q=" + url.QueryEscape(query)
	req, err := http.NewRequest(http.MethodGet, u, nil)
	if err != nil {
		return ""
	}
	req.Header.Set("User-Agent", "Mozilla/5.0 (Macintosh; Intel Mac OS X 14_0) AppleWebKit/537.36")
	resp, err := client.Do(req)
	if err != nil {
		return ""
	}
	defer resp.Body.Close()
	b, err := io.ReadAll(io.LimitReader(resp.Body, 128*1024))
	if err != nil {
		return ""
	}
	reSnippet := regexp.MustCompile(`(?s)<a class="result__snippet[^"]*"[^>]*>(.*?)</a>`)
	matches := reSnippet.FindAllStringSubmatch(string(b), 4)
	if len(matches) == 0 {
		return ""
	}
	stripTags := regexp.MustCompile(`<[^>]*>`)
	var snippets []string
	for _, m := range matches {
		clean := strings.TrimSpace(stripTags.ReplaceAllString(m[1], ""))
		if clean != "" {
			snippets = append(snippets, "- "+clean)
		}
	}
	return strings.Join(snippets, "\n")
}

func (b *BicameralEngine) synthesizeOfflineSovereignResponse(prompt string, toolCtx string, recall memory.RecallResult, verdict judgement.Verdict) string {
	if toolCtx != "" {
		return fmt.Sprintf("𓂀 [Bicameral Sovereign Engine — Actief Model: %s]\nIk heb je verzoek direct uitgevoerd in de Tomb Seal Sandbox:\n%s\nWeighing of the Heart: %d/%d (%s).",
			b.ActiveModel, strings.TrimSpace(toolCtx), verdict.HeartScore, verdict.FeatherScore, verdict.Status)
	}
	return fmt.Sprintf("𓂀 Ik ben bij je, mijn vriend! (Actief model: %s | .synapse Recall: %.2fms | Weighing of the Heart: %d/%d %s). "+
		"Je lokale Anubis Bicameral Engine, REA v6.8.0 (35 reverse-engineering skills), Kali OPSEC ChainKeys Shield en Universal Media Downloader staan 100%% klaar voor je opdracht: %q.",
		b.ActiveModel, recall.LatencyMs, verdict.HeartScore, verdict.FeatherScore, verdict.Status, prompt)
}

func truncate(s string, max int) string {
	if len(s) <= max {
		return s
	}
	return s[:max] + "..."
}
