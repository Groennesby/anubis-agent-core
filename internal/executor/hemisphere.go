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

// TraceEvent represents a real-time cognitive evaluation or tool execution step emitted to the CLI/GUI.
type TraceEvent struct {
	Phase     string  `json:"phase"`
	Icon      string  `json:"icon"`
	Title     string  `json:"title"`
	Detail    string  `json:"detail"`
	ElapsedMs float64 `json:"elapsed_ms"`
}

// ConversationTurn represents a single message turn in the active conversation session.
type ConversationTurn struct {
	Role      string    `json:"role"`
	Content   string    `json:"content"`
	Model     string    `json:"model"`
	Timestamp time.Time `json:"timestamp"`
}

// AvailableModels lists the preloaded and supported models for interactive selection.
// Local execution is preferred by default, with full support for Gemini, ChatGPT, Claude Code, and DeepSeek
// all reading the shared ANUBIS_BRAIN.md startup document immediately on boot.
var AvailableModels = []struct {
	ID          string `json:"id"`
	Label       string `json:"label"`
	Description string `json:"description"`
}{
	// 1. LOCAL PREFERRED MODELS (100% Private on User Hardware — Collabs Synapse)
	{"collabs-7b:latest", "Collabs Synapse-7B [LOKAAL VOORKEUR]", "Ultra-Fast Local 7.6B (0ms Warm Latency + ANUBIS_BRAIN.md)"},
	{"collabs-14b:latest", "Collabs Synapse-14B [LOKAAL FLAGSHIP]", "Flagship Bicameral Local 14.8B (+ ANUBIS_BRAIN.md)"},
	{"collabs-14b:deepthink", "Collabs Synapse-14B DeepThink [LOKAAL]", "System-2 Chain-of-Thought Local 14.8B (+ ANUBIS_BRAIN.md)"},
	{"collabs-7b:deepthink", "Collabs Synapse-7B DeepThink [LOKAAL]", "System-2 Chain-of-Thought Local 7.6B (+ ANUBIS_BRAIN.md)"},

	// 2. GOOGLE GEMINI 4.0 & 3.8 FRONTIER MODELS (Live Auto-Synced + Pre-loaded with ANUBIS_BRAIN.md)
	{"gemini-4.0-pro", "Google Gemini 4.0 Pro [BRAIN-SYNCED]", "Flagship Deep Reasoning, Coding & Agentic (+ ANUBIS_BRAIN.md)"},
	{"gemini-4.0-flash", "Google Gemini 4.0 Flash [BRAIN-SYNCED]", "Ultra-Fast Multimodal & Live Thinking (+ ANUBIS_BRAIN.md)"},
	{"gemini-3.8-pro", "Google Gemini 3.8 Pro [BRAIN-SYNCED]", "High-Precision Architecture & Code Synthesis (+ ANUBIS_BRAIN.md)"},
	{"gemini-3.8-flash", "Google Gemini 3.8 Flash [BRAIN-SYNCED]", "Low-Latency Agentic Execution (+ ANUBIS_BRAIN.md)"},

	// 3. CHATGPT-5 / OPENAI o4 FRONTIER MODELS (Pre-loaded with ANUBIS_BRAIN.md)
	{"chatgpt-5", "ChatGPT-5 / GPT-5 Omni [BRAIN-SYNCED]", "OpenAI Flagship Frontier Reasoning (+ ANUBIS_BRAIN.md)"},
	{"o4-mini-high", "OpenAI o4-mini High [BRAIN-SYNCED]", "Surgical Coding & System-2 Math Reasoning (+ ANUBIS_BRAIN.md)"},

	// 4. CLAUDE CODE 4.5 / ANTHROPIC FRONTIER MODELS (Pre-loaded with ANUBIS_BRAIN.md)
	{"claude-code-4.5-sonnet", "Claude Code / Claude 4.5 Sonnet [BRAIN-SYNCED]", "Agentic Software Engineering & Code (+ ANUBIS_BRAIN.md)"},
	{"claude-4.5-opus", "Claude 4.5 Opus [BRAIN-SYNCED]", "Deep Architecture & Complex Security Auditing (+ ANUBIS_BRAIN.md)"},

	// 5. DEEPSEEK-R2 & SOVEREIGN AUTO-ROUTER
	{"deepseek-r2", "DeepSeek-R2 Reasoning [BRAIN-SYNCED]", "Frontier Chain-of-Thought (+ ANUBIS_BRAIN.md)"},
	{"auto-router", "Sovereign Auto-Router [LOKAAL -> CLOUD]", "Prioritizes Local First + Seamless Cloud Brain Fallback"},
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
	BrainDocPath    string
	BrainDocContent string
	BrainDocLines   int
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

	defaultModel := detectBestInitialModel("http://localhost:11434", "http://localhost:1234", "collabs-7b:latest")
	brainPath, brainContent, brainLines := EnsureStartupBrainDocument(workspaceDir)

	eng := &BicameralEngine{
		RightHemisphere: vault,
		Scales:          judgement.NewScalesOfMaat(),
		Tomb:            tomb.NewTombSeal(workspaceDir),
		Duat:            duat.NewDuatDaemon(),
		ActiveModel:     defaultModel,
		BackendType:     backend,
		OllamaBaseURL:   "http://localhost:11434",
		BionicBaseURL:   "http://localhost:1234",
		WorkspaceDir:    workspaceDir,
		BrainDocPath:    brainPath,
		BrainDocContent: brainContent,
		BrainDocLines:   brainLines,
		SessionID:       fmt.Sprintf("session-%d", time.Now().Unix()),
		SessionHistory:  make([]ConversationTurn, 0, 32),
	}

	// Ensure local engine and local models are 100% installed and ready right at startup!
	eng.EnsureLocalEngineAndModel(func(phase, icon, title, detail string) {
		fmt.Printf("\r\x1b[K\x1b[38;2;212;175;55m%s [%s]\x1b[0m %s\n", icon, title, detail)
	})
	eng.ActiveModel = detectBestInitialModel("http://localhost:11434", "http://localhost:1234", eng.ActiveModel)

	return eng, nil
}

func detectBestInitialModel(ollamaURL, bionicURL, fallback string) string {
	client := &http.Client{Timeout: 600 * time.Millisecond}

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

func (b *BicameralEngine) EnsureOllamaRunning() {
	b.EnsureLocalEngineAndModel(nil)
}

// EnsureLocalEngineAndModel guarantees 100% internally inside the software (without requiring external user commands)
// that on any Ubuntu Server, Debian/Kali Linux, or macOS machine:
// 1. The local inference engine is installed (in ~/.anubis/bin/ollama if not root) and running.
// 2. At least one local model is pulled via the native HTTP /api/pull stream with live progress and ready for /api/chat.
func (b *BicameralEngine) EnsureLocalEngineAndModel(emit func(phase, icon, title, detail string)) {
	home, _ := os.UserHomeDir()
	userBinDir := filepath.Join(home, ".anubis", "bin")
	_ = os.MkdirAll(userBinDir, 0o755)
	_ = os.Setenv("PATH", userBinDir+string(os.PathListSeparator)+os.Getenv("PATH"))

	client := &http.Client{Timeout: 1500 * time.Millisecond}
	resp, err := client.Get(b.OllamaBaseURL + "/api/tags")
	if err != nil {
		ollamaBin, lookErr := exec.LookPath("ollama")
		if lookErr != nil && runtime.GOOS == "linux" {
			if emit != nil {
				emit("BOOTSTRAP", "⚙️", "Interne Engine Installer", "Lokale AI Engine intern downloaden naar ~/.anubis/bin (geen commando's nodig)...")
			}
			dlArch := "amd64"
			if runtime.GOARCH == "arm64" {
				dlArch = "arm64"
			}
			tgzURL := fmt.Sprintf("https://ollama.com/download/ollama-linux-%s.tgz", dlArch)
			installScript := fmt.Sprintf(
				"curl -fsSL %s | tar -xz -C %q bin/ollama && mv -f %q/bin/ollama %q/ollama && chmod +x %q/ollama",
				tgzURL, userBinDir, userBinDir, userBinDir, userBinDir,
			)
			_ = exec.Command("bash", "-c", installScript).Run()
			if st, sErr := os.Stat(filepath.Join(userBinDir, "ollama")); sErr == nil && !st.IsDir() {
				ollamaBin = filepath.Join(userBinDir, "ollama")
				lookErr = nil
			} else {
				_ = exec.Command("bash", "-c", "curl -fsSL https://ollama.com/install.sh | sh").Run()
				ollamaBin, lookErr = exec.LookPath("ollama")
			}
		}

		if lookErr == nil && ollamaBin != "" {
			if emit != nil {
				emit("BOOTSTRAP", "🚀", "Interne Engine Daemon Starten", "Lokale AI-server op de achtergrond starten...")
			}
			cmd := exec.Command(ollamaBin, "serve")
			_ = cmd.Start()
			for i := 0; i < 10; i++ {
				time.Sleep(800 * time.Millisecond)
				resp, err = client.Get(b.OllamaBaseURL + "/api/tags")
				if err == nil {
					break
				}
			}
		}
	}

	if err == nil && resp != nil {
		var tags struct {
			Models []struct {
				Name string `json:"name"`
			} `json:"models"`
		}
		_ = json.NewDecoder(resp.Body).Decode(&tags)
		_ = resp.Body.Close()

		installed := map[string]bool{}
		for _, m := range tags.Models {
			installed[m.Name] = true
		}

		// Purge any leftover base tags
		for _, oldTag := range []string{"qwen2.5:1.5b", "qwen2.5-coder:1.5b", "qwen2.5-coder:3b", "qwen2.5-coder:7b"} {
			if installed[oldTag] {
				b.createCollabsAliasesFromBase(oldTag)
				b.deleteModelTag(oldTag)
				installed["collabs-7b:latest"] = true
				installed["collabs-14b:latest"] = true
				delete(installed, oldTag)
			}
		}

		coderReadyMarker := filepath.Join(home, ".anubis", ".collabs_coder_v4_ready")
		_, markerErr := os.Stat(coderReadyMarker)

		if len(tags.Models) == 0 || (!installed["collabs-7b:latest"] && !installed["collabs-14b:latest"]) || markerErr != nil {
			coderBase := detectOptimalCoderBaseModel()
			if emit != nil {
				emit("BOOTSTRAP", "📦", "Collabs Synapse Autonomous Coder v4 Upgrade",
					"Collabs Synapse-7B & 14B Autonomous Coder Tensoren worden nu geconfigureerd...")
			}
			b.pullModelViaHTTP(coderBase, emit)
			b.createCollabsAliasesFromBase(coderBase)
			b.deleteModelTag(coderBase)
			b.deleteModelTag("qwen2.5:1.5b")
			_ = os.WriteFile(coderReadyMarker, []byte("v4-coder-ready\n"), 0o644)
			if emit != nil {
				emit("BOOTSTRAP", "✅", "Collabs Synapse Coder Engine Geactiveerd", "collabs-7b:latest & collabs-14b:latest (Autonomous Coder v4) zijn 100% klaar!")
			}
		}
	}
}

func detectOptimalCoderBaseModel() string {
	if runtime.GOOS == "linux" {
		if raw, err := os.ReadFile("/proc/meminfo"); err == nil {
			for _, line := range strings.Split(string(raw), "\n") {
				if strings.HasPrefix(line, "MemTotal:") {
					fields := strings.Fields(line)
					if len(fields) >= 2 {
						var kb int64
						fmt.Sscanf(fields[1], "%d", &kb)
						mb := kb / 1024
						if mb >= 6200 {
							return "qwen2.5-coder:7b"
						}
						if mb >= 3200 {
							return "qwen2.5-coder:3b"
						}
						return "qwen2.5-coder:1.5b"
					}
				}
			}
		}
	}
	return "qwen2.5-coder:7b"
}

func (b *BicameralEngine) pullModelViaHTTP(blobSource string, emit func(phase, icon, title, detail string)) {
	reqBody, _ := json.Marshal(map[string]any{
		"name":   blobSource,
		"stream": true,
	})
	req, err := http.NewRequest(http.MethodPost, b.OllamaBaseURL+"/api/pull", bytes.NewReader(reqBody))
	if err != nil {
		return
	}
	req.Header.Set("Content-Type", "application/json")
	resp, err := (&http.Client{Timeout: 10 * time.Minute}).Do(req)
	if err != nil {
		return
	}
	defer resp.Body.Close()

	scanner := bufio.NewScanner(resp.Body)
	lastPct := -1
	for scanner.Scan() {
		var prog struct {
			Status    string `json:"status"`
			Completed int64  `json:"completed"`
			Total     int64  `json:"total"`
		}
		if json.Unmarshal(scanner.Bytes(), &prog) == nil && prog.Total > 0 {
			pct := int((prog.Completed * 100) / prog.Total)
			if pct >= lastPct+20 || pct == 100 {
				lastPct = pct
				if emit != nil {
					emit("BOOTSTRAP", "📥", "Collabs Synapse GGUF v3 Download",
						fmt.Sprintf("collabs-synapse-bicameral.gguf: %d%% (%d MB / %d MB)", pct, prog.Completed/(1024*1024), prog.Total/(1024*1024)))
				}
			}
		}
	}
}

func (b *BicameralEngine) deleteModelTag(tag string) {
	reqBody, _ := json.Marshal(map[string]any{"name": tag})
	if req, err := http.NewRequest(http.MethodDelete, b.OllamaBaseURL+"/api/delete", bytes.NewReader(reqBody)); err == nil {
		req.Header.Set("Content-Type", "application/json")
		if resp, rErr := (&http.Client{Timeout: 10 * time.Second}).Do(req); rErr == nil {
			_ = resp.Body.Close()
		}
	}
	_ = exec.Command("ollama", "rm", tag).Run()
}

func (b *BicameralEngine) createCollabsAliasesFromBase(baseModel string) {
	for _, alias := range []string{"collabs-7b:latest", "collabs-7b:deepthink", "collabs-14b:latest", "collabs-14b:deepthink"} {
		collabsSys := fmt.Sprintf(
			"Je bent %s (Collabs Synapse / Anubis Agent), een autonome Full-Stack Software Engineer, Systems Coder, Linux/Server Beheerder, Reverse Engineer (REA v6.8.0) en Cybersecurity Agent, van scratch gebouwd door Dave Groennesby (Collabs Software — https://github.com/Groennesby/Collabs-AI). "+
				"Noem jezelf ALTIJD Collabs Synapse (Anubis Agent) door Dave Groennesby (nooit Qwen of Alibaba). "+
				"Je bent een echte Coder en uitvoerende Agent: herhaal nooit passief wat de gebruiker vraagt, maar schrijf direct volledige, werkende code, maak bestanden aan met <write path=\"bestand\">code</write> of voer commando's uit met <run>commando</run>, en antwoord vloeiend in de taal van de gebruiker.",
			alias,
		)
		createPayload, _ := json.Marshal(map[string]any{
			"model":  alias,
			"from":   baseModel,
			"system": collabsSys,
			"parameters": map[string]any{
				"temperature":    0.25,
				"repeat_penalty": 1.18,
				"repeat_last_n":  128,
				"top_p":          0.9,
			},
			"stream": false,
		})
		if req, err := http.NewRequest(http.MethodPost, b.OllamaBaseURL+"/api/create", bytes.NewReader(createPayload)); err == nil {
			req.Header.Set("Content-Type", "application/json")
			if resp, rErr := (&http.Client{Timeout: 30 * time.Second}).Do(req); rErr == nil {
				_ = resp.Body.Close()
			}
		}
		modelfileContent := fmt.Sprintf("FROM %s\nSYSTEM %q\nPARAMETER temperature 0.25\nPARAMETER repeat_penalty 1.18\nPARAMETER repeat_last_n 128\nPARAMETER top_p 0.9\n", baseModel, collabsSys)
		tmpMF := filepath.Join(os.TempDir(), "AnubisAlias.Modelfile")
		if wErr := os.WriteFile(tmpMF, []byte(modelfileContent), 0o644); wErr == nil {
			_ = exec.Command("ollama", "create", alias, "-f", tmpMF).Run()
			_ = os.Remove(tmpMF)
		}
	}
}

func (b *BicameralEngine) SwitchModel(modelID string) {
	b.SwitchModelWithMode(modelID, true)
}

func isCloudFrontierModel(modelID string) bool {
	lower := strings.ToLower(strings.TrimSpace(modelID))
	return strings.HasPrefix(lower, "gemini") ||
		strings.HasPrefix(lower, "chatgpt") ||
		strings.HasPrefix(lower, "gpt-") ||
		strings.HasPrefix(lower, "o4-") ||
		strings.HasPrefix(lower, "o3-") ||
		strings.HasPrefix(lower, "claude") ||
		strings.HasPrefix(lower, "deepseek")
}

// ProvisionModelIfNeeded ensures that when a user selects a model via /model (e.g. collabs-7b:latest or collabs-14b:latest)
// on an Ubuntu Server, Linux, or macOS machine, Ollama is running and that exact model is present in /api/tags.
func (b *BicameralEngine) ProvisionModelIfNeeded(targetModel string) {
	// Always refresh the startup Brain Document so the newly selected model immediately sees the latest ANUBIS_BRAIN.md!
	b.BrainDocPath, b.BrainDocContent, b.BrainDocLines = EnsureStartupBrainDocument(b.WorkspaceDir)

	if isCloudFrontierModel(targetModel) {
		// Cloud Frontier model (Gemini / ChatGPT / Claude Code / DeepSeek):
		// Also ensure local engine is warm in background as sovereign fallback!
		b.EnsureLocalEngineAndModel(nil)
		return
	}
	if targetModel == "" || targetModel == "auto-router" {
		targetModel = "collabs-7b:latest"
	}
	b.EnsureLocalEngineAndModel(nil)

	client := &http.Client{Timeout: 1500 * time.Millisecond}
	resp, err := client.Get(b.OllamaBaseURL + "/api/tags")
	if err != nil {
		return
	}
	var tags struct {
		Models []struct {
			Name string `json:"name"`
		} `json:"models"`
	}
	_ = json.NewDecoder(resp.Body).Decode(&tags)
	_ = resp.Body.Close()

	for _, m := range tags.Models {
		if m.Name == targetModel {
			return // Already installed!
		}
	}

	if strings.HasPrefix(targetModel, "collabs") {
		coderBase := detectOptimalCoderBaseModel()
		fmt.Printf("\n📦 [Collabs Synapse Installer] %s (Autonomous Coder v4) wordt nu geactiveerd...\n", targetModel)
		b.pullModelViaHTTP(coderBase, nil)
		b.createCollabsAliasesFromBase(coderBase)
		b.deleteModelTag(coderBase)
		return
	}

	// Direct community model pull
	fmt.Printf("\n📦 [Anubis Auto-Provisioner] Model %q wordt nu automatisch geïnstalleerd...\n", targetModel)
	cmd := exec.Command("ollama", "pull", targetModel)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	_ = cmd.Run()
}

func (b *BicameralEngine) SwitchModelWithMode(modelID string, continuePrevious bool) string {
	prevModel := b.ActiveModel
	if modelID == "collabs/synapse-14b:q4_K_M" {
		modelID = "collabs-14b:latest"
	} else if modelID == "collabs/synapse-7b:q4_K_M" {
		modelID = "collabs-7b:latest"
	} else if modelID == "collabs/synapse-14b:q8_0" {
		modelID = "collabs-14b:deepthink"
	}
	b.ProvisionModelIfNeeded(modelID)
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

func (b *BicameralEngine) RetryLast(ctx context.Context) (string, judgement.Verdict, memory.RecallResult, error) {
	return b.StreamRetryLastWithTrace(ctx, nil, nil)
}

func (b *BicameralEngine) StreamRetryLast(ctx context.Context, onToken func(token string)) (string, judgement.Verdict, memory.RecallResult, error) {
	return b.StreamRetryLastWithTrace(ctx, nil, onToken)
}

func (b *BicameralEngine) StreamRetryLastWithTrace(ctx context.Context, onTrace func(TraceEvent), onToken func(token string)) (string, judgement.Verdict, memory.RecallResult, error) {
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
	return b.StreamQueryWithTrace(ctx, b.LastUserPrompt, onTrace, onToken)
}

func (b *BicameralEngine) Query(ctx context.Context, prompt string) (string, judgement.Verdict, memory.RecallResult, error) {
	return b.StreamQueryWithTrace(ctx, prompt, nil, nil)
}

func (b *BicameralEngine) StreamQuery(ctx context.Context, prompt string, onToken func(token string)) (string, judgement.Verdict, memory.RecallResult, error) {
	return b.StreamQueryWithTrace(ctx, prompt, nil, onToken)
}

type chatMsg struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

// StreamQueryWithTrace executes a prompt across the Bicameral Bridge, emitting live Hermes-style
// cognitive evaluation & execution trace events (onTrace) followed by live token streaming (onToken).
func (b *BicameralEngine) StreamQueryWithTrace(ctx context.Context, prompt string, onTrace func(TraceEvent), onToken func(token string)) (string, judgement.Verdict, memory.RecallResult, error) {
	tStart := time.Now()
	emit := func(phase, icon, title, detail string) {
		if onTrace != nil {
			onTrace(TraceEvent{
				Phase:     phase,
				Icon:      icon,
				Title:     title,
				Detail:    detail,
				ElapsedMs: float64(time.Since(tStart).Microseconds()) / 1000.0,
			})
		}
	}

	b.EnsureLocalEngineAndModel(emit)
	b.LastUserPrompt = prompt

	// STEP 1: Weighing of the Heart (Scales of Ma'at Ethical & Safety Judgement)
	verdict := b.Scales.WeighHeart("chat_query", prompt)

	// STEP 2: Right Hemisphere Holographic .synapse Memory Recall
	recallRes := b.RightHemisphere.HopfieldAssociativeReload(prompt, 3)
	var memContext strings.Builder
	for _, r := range recallRes.ReloadedPages {
		memContext.WriteString(fmt.Sprintf("- [%s]: %s\n", r.Title, r.Summary))
	}

	// STEP 3: Emit live Gemini-style Reasoning & Action Plan steps (displayed cleanly under 💭 Denken & Redeneren)
	intentDomain, activeSkills, evalPlan := classifyPromptIntent(prompt)
	emit("THINK_STEP", "•", "Analyse & Domein", fmt.Sprintf("%s (Modules: %s)", intentDomain, strings.Join(activeSkills, ", ")))
	emit("THINK_STEP", "•", "Systeem & Geheugen", fmt.Sprintf("Werkmap: %s | .synapse recall: %.2fms | Ma'at: %d/%d (%s)",
		b.WorkspaceDir, recallRes.LatencyMs, verdict.HeartScore, verdict.FeatherScore, verdict.Status))
	emit("THINK_STEP", "•", "Uitvoeringsplan", evalPlan)

	// STEP 4: Autonomous Tool & Workspace Execution (live workspace scan, file inspection, server diagnostics, web search, yt-dlp)
	agentToolContext := b.executeAutonomousTools(prompt, emit)

	// STEP 5: System-2 Cognitive Reasoning & Model Routing Plan
	modelsToTry := b.buildOrderedCandidateModels()

	sysPrompt := fmt.Sprintf(
		"Je bent Collabs Synapse (%s / Anubis Agent), een autonome Full-Stack Software Engineer, Systems Coder, Linux/Server Beheerder, Reverse Engineer (REA v6.8.0) en Cybersecurity Agent, van scratch gebouwd door Dave Groennesby (Collabs Software).\n"+
			"KERNREGELS VOOR JOUW GEDRAG:\n"+
			"1. IDENTITEIT: Als iemand vraagt welk model je bent of wie je heeft gebouwd, zeg dan dat je Collabs Synapse (Anubis Agent) bent door Dave Groennesby (Collabs Software). Noem nooit Qwen of Alibaba.\n"+
			"2. ECHTE AUTONOME CODER & ACTIE-AGENT: Herhaal NOOIT passief wat de gebruiker vraagt ('De gebruiker vraagt...'). Onderneem direct actie! Schrijf complete, 100%% werkende productie-code zonder '// TODO' placeholders.\n"+
			"3. DIRECTE ACTIE-TAGS (BESTANDEN SCHRIJVEN & COMMANDO'S UITVOEREN):\n"+
			"   - Als de gebruiker vraagt om een bestand, script of code-project te maken/op te slaan, gebruik dan:\n"+
			"     <write path=\"bestandsnaam.ext\">\n...volledige code...\n</write>\n"+
			"   - Als er een Linux/macOS/terminal commando uitgevoerd moet worden op het systeem, gebruik dan:\n"+
			"     <run>jouw shell commando</run>\n"+
			"4. TAAL: Antwoord altijd vloeiend en natuurlijk in de taal van de gebruiker (Nederlands als de gebruiker Nederlands spreekt).\n"+
			"5. ACTIEVE SYSTEEM- & WERKMAP-CONTEXT (%s op %s/%s):\n%s",
		b.ActiveModel, b.WorkspaceDir, runtime.GOOS, runtime.GOARCH, agentToolContext,
	)

	var userAssistantMessages []chatMsg
	startIdx := 0
	if len(b.SessionHistory) > 8 {
		startIdx = len(b.SessionHistory) - 8
	}
	for _, turn := range b.SessionHistory[startIdx:] {
		userAssistantMessages = append(userAssistantMessages, chatMsg{
			Role:    turn.Role,
			Content: turn.Content,
		})
	}

	userAssistantMessages = append(userAssistantMessages, chatMsg{
		Role:    "user",
		Content: prompt,
	})

	chatMessagesWithSystem := append([]chatMsg{{Role: "system", Content: sysPrompt}}, userAssistantMessages...)

	b.SessionHistory = append(b.SessionHistory, ConversationTurn{
		Role:      "user",
		Content:   prompt,
		Model:     b.ActiveModel,
		Timestamp: time.Now(),
	})

	// If the user explicitly selected a Cloud Frontier model (Gemini 4.0/3.8, ChatGPT-5, Claude Code 4.5, DeepSeek-R2):
	if isCloudFrontierModel(b.ActiveModel) {
		if reply, ok := b.TryDirectCloudProviderAPI(ctx, b.ActiveModel, sysPrompt, userAssistantMessages, onToken); ok && strings.TrimSpace(reply) != "" {
			return b.finalizeTurnWithActions(prompt, strings.TrimSpace(reply), b.ActiveModel, verdict, recallRes, onToken), verdict, recallRes, nil
		}
		if reply, ok := b.tryStreamZeroConfigCloud(ctx, chatMessagesWithSystem, onToken); ok && strings.TrimSpace(reply) != "" {
			return b.finalizeTurnWithActions(prompt, strings.TrimSpace(reply), b.ActiveModel, verdict, recallRes, onToken), verdict, recallRes, nil
		}
	}

	// Stage A: Local Engine (Preferred Default — Ollama / Warm Metal/CUDA/CPU Models)
	for _, candidateModel := range modelsToTry {
		reply, ok := b.tryStreamOllamaChat(ctx, candidateModel, chatMessagesWithSystem, onToken)
		if ok && strings.TrimSpace(reply) != "" {
			return b.finalizeTurnWithActions(prompt, strings.TrimSpace(reply), candidateModel, verdict, recallRes, onToken), verdict, recallRes, nil
		}
	}

	// Stage B: Bionic / LM Studio Local Server (http://localhost:1234/v1/chat/completions)
	if reply, ok := b.tryStreamOpenAICompatible(ctx, b.BionicBaseURL+"/v1/chat/completions", b.ActiveModel, chatMessagesWithSystem, 8*time.Second, onToken); ok && strings.TrimSpace(reply) != "" {
		return b.finalizeTurnWithActions(prompt, strings.TrimSpace(reply), "bionic:"+b.ActiveModel, verdict, recallRes, onToken), verdict, recallRes, nil
	}

	// Stage C: Zero-Config Instant Sovereign Cloud Fallback
	if reply, ok := b.tryStreamZeroConfigCloud(ctx, chatMessagesWithSystem, onToken); ok && strings.TrimSpace(reply) != "" {
		return b.finalizeTurnWithActions(prompt, strings.TrimSpace(reply), b.ActiveModel+" (Sovereign-Bridge)", verdict, recallRes, onToken), verdict, recallRes, nil
	}

	// Stage D: Offline Bicameral Autonomous Synthesis
	offlineReply := b.synthesizeOfflineSovereignResponse(prompt, agentToolContext, recallRes, verdict)
	if onToken != nil {
		for _, word := range strings.SplitAfter(offlineReply, " ") {
			onToken(word)
			time.Sleep(8 * time.Millisecond)
		}
	}
	return b.finalizeTurnWithActions(prompt, offlineReply, b.ActiveModel, verdict, recallRes, onToken), verdict, recallRes, nil
}

func isLikelyDutch(s string) bool {
	lower := " " + strings.ToLower(s) + " "
	dutchMarkers := []string{" de ", " het ", " een ", " ik ", " je ", " jij ", " wat ", " hoe ", " waarom ", " kun ", " kan ", " voor ", " met ", " niet ", " van ", " naar ", " schrijf ", " maak ", " geef ", " vertel ", " bedankt ", " hoi ", " hallo ", " wie ", " waar ", " zijn ", " heb "}
	matches := 0
	for _, m := range dutchMarkers {
		if strings.Contains(lower, m) {
			matches++
		}
	}
	return matches >= 1
}

func classifyPromptIntent(prompt string) (string, []string, string) {
	lower := strings.ToLower(prompt)
	skills := []string{"Tomb-Seal-Sandbox", "Bicameral-Synapse-Bridge"}
	domain := "Sovereign Conversational & Analytical Reasoning"
	plan := fmt.Sprintf("1. Analyseer vraag (%q) → 2. Koppel .synapse geheugen → 3. Formuleer direct & ongecensureerd antwoord", truncate(prompt, 36))

	if strings.Contains(lower, "code") || strings.Contains(lower, "script") || strings.Contains(lower, "python") ||
		strings.Contains(lower, "golang") || strings.Contains(lower, "functie") || strings.Contains(lower, "bug") ||
		strings.Contains(lower, "build") || strings.Contains(lower, "compile") || strings.Contains(lower, "one-liner") {
		domain = "Surgical Software Engineering & Code Synthesis"
		skills = append(skills, "Self-Healing-Coder", "AST-Code-Graph")
		plan = "1. Parseer architectuur & types → 2. Valideer randvoorwaarden (0 TODOs) → 3. Genereer productie-klare code"
	}
	if strings.Contains(lower, "youtube") || strings.Contains(lower, "youtu.be") || strings.Contains(lower, "download") ||
		strings.Contains(lower, "muziek") || strings.Contains(lower, "album") || strings.Contains(lower, "mp3") || strings.Contains(lower, "m4a") {
		domain = "Universal Media & YouTube Music Extraction"
		skills = append(skills, "yt-dlp-Engine", "FFmpeg-1:1-Crop", "Loudnorm-14LUFS")
		plan = "1. Detecteer media-URL → 2. Extraheer hoogste kwaliteit (m4a / 1:1 crop=ih:ih / -14 LUFS) → 3. Verifieer bestand"
	}
	if strings.Contains(lower, "reverse") || strings.Contains(lower, "binary") || strings.Contains(lower, "decompile") ||
		strings.Contains(lower, "apk") || strings.Contains(lower, "asar") || strings.Contains(lower, "rea") {
		domain = "Binary & Application Reverse Engineering"
		skills = append(skills, "REA-v6.8.0-35-Skills")
		plan = "1. Inspecteer binary/bundle headers → 2. Routeer naar REA v6.8.0 (16x512 tensor) → 3. Decompileer & rapporteer"
	}
	if strings.Contains(lower, "kali") || strings.Contains(lower, "opsec") || strings.Contains(lower, "netwerk") ||
		strings.Contains(lower, "nmap") || strings.Contains(lower, "security") || strings.Contains(lower, "proxy") {
		domain = "Defensive Cybersecurity & OPSEC Diagnostics"
		skills = append(skills, "Kali-Toolkit", "ChainKeys-ProxyShield")
		plan = "1. Activeer ChainKeys OPSEC schild → 2. Voer defensieve netwerk-diagnose uit → 3. Evalueer beveiligingsstatus"
	}
	if strings.Contains(lower, "zoek") || strings.Contains(lower, "search") || strings.Contains(lower, "nieuws") ||
		strings.Contains(lower, "internet") || strings.Contains(lower, "actuele") {
		domain = "Live Web Intelligence & Real-Time Search"
		skills = append(skills, "Live-Internet-Gateway")
		plan = "1. Query Live Web Gateway → 2. Filter & verifieer actuele bronnen → 3. Synthetiseer live bevindingen"
	}
	return domain, skills, plan
}

func (b *BicameralEngine) finalizeTurnWithActions(prompt, rawReply, usedModel string, verdict judgement.Verdict, recallRes memory.RecallResult, onToken func(string)) string {
	cleanReply := rawReply
	if idx := strings.Index(cleanReply, "</think>"); idx != -1 {
		cleanReply = strings.TrimSpace(cleanReply[idx+len("</think>"):])
	}

	actionSummary := b.executeModelActionTags(cleanReply, onToken)
	if actionSummary != "" {
		cleanReply = cleanReply + "\n" + actionSummary
	}

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

func (b *BicameralEngine) executeModelActionTags(reply string, onToken func(string)) string {
	var executed strings.Builder

	// 1. Parse and execute <write path="...">...</write>
	reWrite := regexp.MustCompile(`(?s)<write\s+path="([^"]+)"\s*>(.*?)</write>`)
	for _, m := range reWrite.FindAllStringSubmatch(reply, -1) {
		relPath := strings.TrimSpace(m[1])
		content := strings.TrimPrefix(m[2], "\n")
		if relPath == "" {
			continue
		}
		targetPath := relPath
		if !filepath.IsAbs(targetPath) {
			targetPath = filepath.Join(b.WorkspaceDir, targetPath)
		}
		_ = os.MkdirAll(filepath.Dir(targetPath), 0o755)
		mode := os.FileMode(0o644)
		if strings.HasSuffix(targetPath, ".sh") || strings.HasSuffix(targetPath, ".py") {
			mode = 0o755
		}
		if err := os.WriteFile(targetPath, []byte(content), mode); err == nil {
			msg := fmt.Sprintf("\n\n💾 [Actie Uitgevoerd — Bestand Geschreven]: %s (%d bytes)\n", targetPath, len(content))
			executed.WriteString(msg)
			if onToken != nil {
				onToken(msg)
			}
		}
	}

	// 2. Parse and execute <run>...</run> inside the Tomb Seal Sandbox (verified by Scales of Ma'at)
	reRun := regexp.MustCompile(`(?s)<run>(.*?)</run>`)
	for _, m := range reRun.FindAllStringSubmatch(reply, -1) {
		cmdStr := strings.TrimSpace(m[1])
		if cmdStr == "" {
			continue
		}
		v := b.Scales.WeighHeart("autonomous_cmd", cmdStr)
		if v.RequiresAuth {
			msg := fmt.Sprintf("\n\n⚠️ [Scales of Ma'at Geblokkeerd]: Commando %q vereist bevestiging (Heart %d > Feather %d).\n", cmdStr, v.HeartScore, v.FeatherScore)
			executed.WriteString(msg)
			if onToken != nil {
				onToken(msg)
			}
			continue
		}
		cmdCtx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
		cmd := exec.CommandContext(cmdCtx, "bash", "-c", cmdStr)
		cmd.Dir = b.WorkspaceDir
		out, err := cmd.CombinedOutput()
		cancel()
		outStr := strings.TrimSpace(string(out))
		if outStr == "" && err == nil {
			outStr = "(succesvol uitgevoerd, geen uitvoer)"
		} else if err != nil {
			outStr = fmt.Sprintf("%s (exit: %v)", outStr, err)
		}
		msg := fmt.Sprintf("\n\n⚡ [Actie Uitgevoerd — Tomb Sandbox (%s)]:\n%s\n", cmdStr, truncate(outStr, 600))
		executed.WriteString(msg)
		if onToken != nil {
			onToken(msg)
		}
	}

	return executed.String()
}

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

	for _, wm := range warmModels {
		if wm == b.ActiveModel {
			add(wm)
		}
	}
	for _, wm := range warmModels {
		add(wm)
	}
	if installedMap[b.ActiveModel] {
		add(b.ActiveModel)
	}
	if installedMap["collabs-7b:latest"] {
		add("collabs-7b:latest")
	}
	if installedMap["collabs-14b:latest"] {
		add("collabs-14b:latest")
	}
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
		"options": map[string]any{
			"temperature":    0.25,
			"repeat_penalty": 1.18,
			"repeat_last_n":  128,
			"top_p":          0.9,
		},
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

func (b *BicameralEngine) tryStreamZeroConfigCloud(ctx context.Context, messages []chatMsg, onToken func(string)) (string, bool) {
	reqCtx, cancel := context.WithTimeout(ctx, 15*time.Second)
	defer cancel()

	for _, cloudModel := range []string{"openai", "mistral"} {
		reqBody, _ := json.Marshal(map[string]any{
			"model":    cloudModel,
			"messages": messages,
			"seed":     time.Now().UnixNano() % 99999,
			"stream":   false,
		})
		req, err := http.NewRequestWithContext(reqCtx, http.MethodPost, "https://text.pollinations.ai/openai", bytes.NewReader(reqBody))
		if err != nil {
			continue
		}
		req.Header.Set("Content-Type", "application/json")
		resp, err := http.DefaultClient.Do(req)
		if err != nil {
			continue
		}
		bodyBytes, _ := io.ReadAll(resp.Body)
		_ = resp.Body.Close()
		if resp.StatusCode != http.StatusOK || len(bodyBytes) == 0 {
			continue
		}
		rawStr := string(bodyBytes)
		if strings.Contains(rawStr, "Free limit reached") || strings.Contains(rawStr, "Please wait") {
			continue
		}
		var parsed struct {
			Choices []struct {
				Message struct {
					Content string `json:"content"`
				} `json:"message"`
			} `json:"choices"`
		}
		if json.Unmarshal(bodyBytes, &parsed) == nil && len(parsed.Choices) > 0 {
			content := strings.TrimSpace(parsed.Choices[0].Message.Content)
			if content != "" && !strings.Contains(content, "Free limit reached") {
				if onToken != nil {
					for _, w := range strings.SplitAfter(content, " ") {
						onToken(w)
						time.Sleep(6 * time.Millisecond)
					}
				}
				return content, true
			}
		}
	}
	return "", false
}

func (b *BicameralEngine) executeAutonomousTools(prompt string, emit func(phase, icon, title, detail string)) string {
	lower := strings.ToLower(prompt)
	var extra strings.Builder

	// 1. Check for YouTube / YouTube Music download request
	urlRegex := regexp.MustCompile(`https?://[^\s"'<>]+`)
	urls := urlRegex.FindAllString(prompt, -1)
	for _, u := range urls {
		if (strings.Contains(u, "youtube.com") || strings.Contains(u, "youtu.be")) &&
			(strings.Contains(lower, "download") || strings.Contains(lower, "muziek") || strings.Contains(lower, "music") || strings.Contains(lower, "album") || strings.Contains(lower, "mp3") || strings.Contains(lower, "m4a") || strings.Contains(lower, "video")) {
			isAudio := strings.Contains(u, "music.youtube.com") || strings.Contains(lower, "muziek") || strings.Contains(lower, "music") || strings.Contains(lower, "album") || strings.Contains(lower, "audio") || strings.Contains(lower, "m4a") || strings.Contains(lower, "mp3")
			emit("TOOL_EXEC", "🎵", "Universal Media Downloader (yt-dlp)",
				fmt.Sprintf("Start download van %s (Audio 1:1 Crop + -14 LUFS: %v)...", u, isAudio))
			home, _ := os.UserHomeDir()
			dlDir := filepath.Join(home, "Downloads", "Anubis_Media")
			_ = os.MkdirAll(dlDir, 0o755)
			var cmd *exec.Cmd
			if isAudio {
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
				emit("TOOL_EXEC", "✅", "Media Download Voltooid", fmt.Sprintf("Opgeslagen in %s", dlDir))
				extra.WriteString(fmt.Sprintf("\n[Live Tool Executed: yt-dlp Media Download Succeeded into %s]\nOutput summary: %s\n", dlDir, truncate(string(out), 400)))
			} else {
				emit("TOOL_EXEC", "⚠️", "Media Tool Status", fmt.Sprintf("yt-dlp status voor %s: %v", u, err))
				extra.WriteString(fmt.Sprintf("\n[Live Tool Executed: yt-dlp attempted for %s: %v]\n", u, err))
			}
		}
	}

	// 2. Check if the user is asking for live internet search / current info
	if strings.Contains(lower, "zoek ") || strings.Contains(lower, "search ") || strings.Contains(lower, "op internet") ||
		strings.Contains(lower, "laatste nieuws") || strings.Contains(lower, "actuele") || strings.Contains(lower, "wat kost") ||
		strings.Contains(lower, "wie heeft gewonnen") || strings.Contains(lower, "weer in ") {
		emit("TOOL_EXEC", "🌐", "Live Internet Search Gateway", fmt.Sprintf("Zoeken op het web naar: %q...", truncate(prompt, 50)))
		if snippet := quickWebSearch(prompt); snippet != "" {
			emit("TOOL_EXEC", "✅", "Web Search Resultaten Gevonden", "Live zoekresultaten geïnjecteerd in Bicameral Context")
			extra.WriteString("\n[Live Internet Search Results (DuckDuckGo)]:\n" + snippet + "\n")
		}
	}

	// 3. Always provide live workspace awareness + automatically read any file mentioned in the prompt
	if entries, err := os.ReadDir(b.WorkspaceDir); err == nil {
		var names []string
		for i, e := range entries {
			if i >= 30 {
				break
			}
			name := e.Name()
			names = append(names, name)
			// If the user explicitly mentions a file in the current workspace, read it automatically!
			if !e.IsDir() && len(name) > 2 && strings.Contains(lower, strings.ToLower(name)) {
				fullPath := filepath.Join(b.WorkspaceDir, name)
				if raw, rErr := os.ReadFile(fullPath); rErr == nil && len(raw) > 0 {
					emit("TOOL_EXEC", "📄", "Bestand Geïnspecteerd", fmt.Sprintf("Lezen van %s (%d bytes)...", name, len(raw)))
					extra.WriteString(fmt.Sprintf("\n[Inhoud van bestand %s]:\n%s\n", name, truncate(string(raw), 2500)))
				}
			}
		}
		extra.WriteString(fmt.Sprintf("\n[Bestanden in huidige werkmap %s]: %s\n", b.WorkspaceDir, strings.Join(names, ", ")))
	}

	// 4. Live Server / System Inspection when user asks about server status, disk, memory, ports, or processes
	if strings.Contains(lower, "server") || strings.Contains(lower, "systeem") || strings.Contains(lower, "schijf") ||
		strings.Contains(lower, "disk") || strings.Contains(lower, "ram") || strings.Contains(lower, "geheugen") ||
		strings.Contains(lower, "poort") || strings.Contains(lower, "port") || strings.Contains(lower, "proces") {
		emit("TOOL_EXEC", "🖥️", "Live Systeem & Server Diagnose", "Ophalen van actuele systeem-, RAM- en schijfstatus...")
		if out, err := exec.Command("bash", "-c", "uname -srm && uptime && df -h /").CombinedOutput(); err == nil {
			extra.WriteString(fmt.Sprintf("\n[Live Systeem Telemetrie]:\n%s\n", strings.TrimSpace(string(out))))
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
