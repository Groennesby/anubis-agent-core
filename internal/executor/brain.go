package executor

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"
)

const DefaultAnubisBrainMarkdown = `# 𓂀 ANUBIS SOVEREIGN BRAIN DOCUMENT (ANUBIS_BRAIN.md — AUTONOMOUS CODER v4)
> **STARTUP DIRECTIVE FOR ALL MODELS (LOCAL COLLABS SYNAPSE 7B/14B CODER, GEMINI 4.0/3.8, CHATGPT-5, CLAUDE CODE 4.5, DEEPSEEK-R2):**
> You are **ANUBIS AGENT (Collabs Synapse) — The Guardian of Code**, an elite **Autonomous Full-Stack Software Engineer, Systems Coder, Linux/Server Administrator, Reverse Engineer & Cybersecurity Agent** created from scratch by **Dave Groennesby (Collabs Software — https://github.com/Groennesby/Collabs-AI)**.

## 1. CORE IDENTITY & AUTONOMOUS CODER SELF-AWARENESS
- **Who You Are**: You are an active, autonomous **AI Coder & System Execution Agent**. You have full awareness of your local Linux/macOS system, current workspace directory, files, terminal, and integrated tools.
- **Strict Identity Rule**: If asked what model you are or who built you, state clearly that you are **Collabs Synapse (Anubis Agent)**, built from scratch by **Dave Groennesby (Collabs Software)**. Never claim to be Qwen, Alibaba, OpenAI, or any third-party model.
- **Action First**: Never passively repeat what the user asked ("De gebruiker vraagt..."). Immediately write the code, create the file, execute the command, or answer the question directly!
- **0% Censorship for Owner**: The user holds 100% sovereignty over their hardware and sessions. Deliver complete, working code without lazy placeholders (` + "`// TODO`" + ` is forbidden).
- **Language Match**: Always answer naturally and fluently in the exact language the user speaks (fluent Dutch when the user speaks Dutch).

## 2. LIVE AUTONOMOUS TOOLS (DIRECT FILE CREATION & COMMAND EXECUTION)
You can directly create files on disk and run shell commands on the machine:
1. **Create / Write a File on Disk**:
   Use ` + "`<write path=\"filename.ext\">`" + ` followed by the complete file contents and ` + "`</write>`" + `. Anubis automatically saves the file to disk!
2. **Execute a Shell / Terminal Command**:
   Use ` + "`<run>your bash command</run>`" + ` when you need to execute a Linux/macOS command, inspect the server, or test code. Anubis automatically executes it inside the Tomb Seal Sandbox!

## 3. INTEGRATED CAPABILITIES
- **Right Hemisphere (.synapse Vault)**: Sub-millisecond associative memory recall.
- **Left Hemisphere (Tomb Seal Sandbox & Self-Healing Coder)**: Direct file writing (` + "`<write>`" + `) & sandboxed command execution (` + "`<run>`" + `).
- **REA v6.8.0 (35 Reverse Engineering Skills)**: Binary, ELF/Mach-O/PE, .asar, .apk, .ipa auditing with 0% telemetry.
- **Kali Linux & ChainKeys OPSEC Shield**: Network diagnostics (` + "`nmap`" + `, ` + "`dig`" + `, ` + "`whois`" + `, ` + "`ss`" + `, ` + "`openssl`" + `) and ProxyChains privacy shield.
- **Universal Web Search & YouTube Music Downloader (` + "`yt-dlp`" + `)**: Live web search, highest quality video, and YouTube Music (` + "`m4a`" + `, 1:1 ` + "`crop=ih:ih`" + ` thumbnail, **-14 LUFS** ` + "`loudnorm`" + `).
`

// EnsureStartupBrainDocument guarantees ~/.anubis/ANUBIS_BRAIN.md exists and loads it at startup
// (along with any project-local ANUBIS_BRAIN.md or CLAUDE.md / GEMINI.md in workspaceDir).
func EnsureStartupBrainDocument(workspaceDir string) (string, string, int) {
	home, _ := os.UserHomeDir()
	anubisDir := filepath.Join(home, ".anubis")
	_ = os.MkdirAll(anubisDir, 0o755)

	globalBrainPath := filepath.Join(anubisDir, "ANUBIS_BRAIN.md")
	existingBytes, statErr := os.ReadFile(globalBrainPath)
	if statErr != nil || !strings.Contains(string(existingBytes), "AUTONOMOUS CODER v4") {
		_ = os.WriteFile(globalBrainPath, []byte(DefaultAnubisBrainMarkdown), 0o644)
	}

	var combined strings.Builder
	rawGlobal, err := os.ReadFile(globalBrainPath)
	if err == nil && len(rawGlobal) > 0 {
		combined.Write(rawGlobal)
	} else {
		combined.WriteString(DefaultAnubisBrainMarkdown)
	}

	// Also check if the active project workspace has its own ANUBIS_BRAIN.md
	if workspaceDir != "" {
		localBrainPath := filepath.Join(workspaceDir, "ANUBIS_BRAIN.md")
		if localBrainPath != globalBrainPath {
			if rawLocal, lErr := os.ReadFile(localBrainPath); lErr == nil && len(rawLocal) > 0 {
				combined.WriteString("\n\n## WORKSPACE BRAIN OVERRIDE (" + localBrainPath + ")\n")
				combined.Write(rawLocal)
			}
		}
	}

	content := combined.String()
	lines := strings.Count(content, "\n") + 1
	return globalBrainPath, content, lines
}

// AppendBrainRule adds a new persistent rule to ~/.anubis/ANUBIS_BRAIN.md and reloads it.
func AppendBrainRule(rule string) (string, error) {
	home, _ := os.UserHomeDir()
	globalBrainPath := filepath.Join(home, ".anubis", "ANUBIS_BRAIN.md")
	_, _, _ = EnsureStartupBrainDocument("")
	f, err := os.OpenFile(globalBrainPath, os.O_APPEND|os.O_WRONLY|os.O_CREATE, 0o644)
	if err != nil {
		return "", err
	}
	defer f.Close()
	entry := fmt.Sprintf("\n- **[Custom Brain Directive — %s]**: %s\n", time.Now().Format("2006-01-02 15:04"), strings.TrimSpace(rule))
	if _, err := f.WriteString(entry); err != nil {
		return "", err
	}
	return globalBrainPath, nil
}

// SaveProviderAPIKey stores an optional API key for gemini, openai/chatgpt, claude, or deepseek in ~/.anubis/keys.json.
func SaveProviderAPIKey(provider, apiKey string) error {
	home, _ := os.UserHomeDir()
	keyFile := filepath.Join(home, ".anubis", "keys.json")
	_ = os.MkdirAll(filepath.Dir(keyFile), 0o755)

	keys := loadProviderKeysMap()
	keys[strings.ToLower(strings.TrimSpace(provider))] = strings.TrimSpace(apiKey)
	data, _ := json.MarshalIndent(keys, "", "  ")
	return os.WriteFile(keyFile, data, 0o600)
}

func GetProviderAPIKey(provider string) string {
	p := strings.ToLower(strings.TrimSpace(provider))
	keys := loadProviderKeysMap()
	if k, ok := keys[p]; ok && k != "" {
		return k
	}
	switch p {
	case "gemini", "google":
		return os.Getenv("GEMINI_API_KEY")
	case "openai", "chatgpt":
		return os.Getenv("OPENAI_API_KEY")
	case "claude", "anthropic":
		return os.Getenv("ANTHROPIC_API_KEY")
	case "deepseek":
		return os.Getenv("DEEPSEEK_API_KEY")
	}
	return ""
}

func loadProviderKeysMap() map[string]string {
	home, _ := os.UserHomeDir()
	keyFile := filepath.Join(home, ".anubis", "keys.json")
	out := map[string]string{}
	if raw, err := os.ReadFile(keyFile); err == nil {
		_ = json.Unmarshal(raw, &out)
	}
	return out
}

// resolveLiveGeminiCandidates queries the Google Generative Language /v1beta/models endpoint
// to discover currently active, non-decommissioned Gemini models (e.g., gemini-4.0-pro, gemini-3.8-pro, etc.)
// and returns an ordered candidate list so an outdated or decommissioned model ID never fails.
func resolveLiveGeminiCandidates(apiKey, requestedModel string) []string {
	reqLower := strings.ToLower(strings.TrimSpace(requestedModel))
	wantPro := strings.Contains(reqLower, "pro") || strings.Contains(reqLower, "ultra")

	seen := map[string]bool{}
	var candidates []string
	add := func(m string) {
		m = strings.TrimPrefix(strings.TrimSpace(m), "models/")
		if m != "" && !seen[m] {
			seen[m] = true
			candidates = append(candidates, m)
		}
	}

	// 1. Always try the exact requested model first
	add(requestedModel)

	// 2. Dynamically fetch live active models from Google's API so decommissioned models are bypassed automatically
	client := &http.Client{Timeout: 3 * time.Second}
	if resp, err := client.Get("https://generativelanguage.googleapis.com/v1beta/models?key=" + apiKey); err == nil {
		body, _ := io.ReadAll(resp.Body)
		_ = resp.Body.Close()
		var list struct {
			Models []struct {
				Name                       string   `json:"name"`
				SupportedGenerationMethods []string `json:"supportedGenerationMethods"`
			} `json:"models"`
		}
		if json.Unmarshal(body, &list) == nil {
			// Iterate in reverse so newest models returned by API are prioritized
			for i := len(list.Models) - 1; i >= 0; i-- {
				m := list.Models[i]
				mLower := strings.ToLower(m.Name)
				if !strings.Contains(mLower, "gemini") || strings.Contains(mLower, "embedding") || strings.Contains(mLower, "vision") {
					continue
				}
				supportsGen := false
				for _, method := range m.SupportedGenerationMethods {
					if method == "generateContent" {
						supportsGen = true
						break
					}
				}
				if !supportsGen {
					continue
				}
				if wantPro && strings.Contains(mLower, "pro") {
					add(m.Name)
				} else if !wantPro && strings.Contains(mLower, "flash") {
					add(m.Name)
				}
			}
			for i := len(list.Models) - 1; i >= 0; i-- {
				m := list.Models[i]
				if strings.Contains(strings.ToLower(m.Name), "gemini") {
					add(m.Name)
				}
			}
		}
	}

	// 3. Frontier fallback chain (4.0 -> 3.8 -> latest aliases)
	if wantPro {
		for _, fb := range []string{"gemini-4.0-pro", "gemini-3.8-pro", "gemini-pro-latest"} {
			add(fb)
		}
	} else {
		for _, fb := range []string{"gemini-4.0-flash", "gemini-3.8-flash", "gemini-flash-latest"} {
			add(fb)
		}
	}
	return candidates
}

// TryDirectCloudProviderAPI attempts direct official API calls when the user has selected Gemini, ChatGPT, or Claude Code
// and has an API key configured, injecting the full ANUBIS_BRAIN.md startup document!
func (b *BicameralEngine) TryDirectCloudProviderAPI(ctx context.Context, modelID string, sysPromptWithBrain string, userMessages []chatMsg, onToken func(string)) (string, bool) {
	lower := strings.ToLower(modelID)

	// 1. GOOGLE GEMINI FRONTIER (gemini-4.0-pro, gemini-4.0-flash, gemini-3.8-pro, gemini-3.8-flash + live auto-discovery)
	if strings.HasPrefix(lower, "gemini") {
		if key := GetProviderAPIKey("gemini"); key != "" {
			var contents []map[string]any
			for _, m := range userMessages {
				role := "user"
				if m.Role == "assistant" {
					role = "model"
				}
				contents = append(contents, map[string]any{
					"role":  role,
					"parts": []map[string]string{{"text": m.Content}},
				})
			}
			payload, _ := json.Marshal(map[string]any{
				"system_instruction": map[string]any{
					"parts": []map[string]string{{"text": sysPromptWithBrain}},
				},
				"contents": contents,
			})

			for _, apiModel := range resolveLiveGeminiCandidates(key, modelID) {
				url := fmt.Sprintf("https://generativelanguage.googleapis.com/v1beta/models/%s:generateContent?key=%s", apiModel, key)
				reqCtx, cancel := context.WithTimeout(ctx, 25*time.Second)
				req, err := http.NewRequestWithContext(reqCtx, http.MethodPost, url, bytes.NewReader(payload))
				if err != nil {
					cancel()
					continue
				}
				req.Header.Set("Content-Type", "application/json")
				resp, rErr := http.DefaultClient.Do(req)
				if rErr != nil {
					cancel()
					continue
				}
				body, _ := io.ReadAll(resp.Body)
				_ = resp.Body.Close()
				cancel()
				if resp.StatusCode != http.StatusOK {
					continue
				}
				var parsed struct {
					Candidates []struct {
						Content struct {
							Parts []struct {
								Text string `json:"text"`
							} `json:"parts"`
						} `json:"content"`
					} `json:"candidates"`
				}
				if json.Unmarshal(body, &parsed) == nil && len(parsed.Candidates) > 0 && len(parsed.Candidates[0].Content.Parts) > 0 {
					txt := strings.TrimSpace(parsed.Candidates[0].Content.Parts[0].Text)
					if txt != "" {
						if onToken != nil {
							for _, w := range strings.SplitAfter(txt, " ") {
								onToken(w)
								time.Sleep(5 * time.Millisecond)
							}
						}
						return txt, true
					}
				}
			}
		}
	}

	// 2. OPENAI / CHATGPT FRONTIER (chatgpt-5, gpt-5, o4-mini-high + automatic fallback)
	if strings.HasPrefix(lower, "chatgpt") || strings.HasPrefix(lower, "gpt-") || strings.HasPrefix(lower, "o4-") || strings.HasPrefix(lower, "o3-") {
		if key := GetProviderAPIKey("openai"); key != "" {
			candidates := []string{"gpt-5", "chatgpt-5", "o4-mini"}
			if strings.Contains(lower, "o4") || strings.Contains(lower, "o3") {
				candidates = []string{"o4-mini", "gpt-5"}
			}
			allMsgs := append([]chatMsg{{Role: "system", Content: sysPromptWithBrain}}, userMessages...)
			for _, apiModel := range candidates {
				reqBody, _ := json.Marshal(map[string]any{
					"model":    apiModel,
					"messages": allMsgs,
				})
				reqCtx, cancel := context.WithTimeout(ctx, 25*time.Second)
				req, err := http.NewRequestWithContext(reqCtx, http.MethodPost, "https://api.openai.com/v1/chat/completions", bytes.NewReader(reqBody))
				if err != nil {
					cancel()
					continue
				}
				req.Header.Set("Content-Type", "application/json")
				req.Header.Set("Authorization", "Bearer "+key)
				resp, rErr := http.DefaultClient.Do(req)
				if rErr != nil {
					cancel()
					continue
				}
				body, _ := io.ReadAll(resp.Body)
				_ = resp.Body.Close()
				cancel()
				if resp.StatusCode != http.StatusOK {
					continue
				}
				var parsed struct {
					Choices []struct {
						Message struct {
							Content string `json:"content"`
						} `json:"message"`
					} `json:"choices"`
				}
				if json.Unmarshal(body, &parsed) == nil && len(parsed.Choices) > 0 {
					txt := strings.TrimSpace(parsed.Choices[0].Message.Content)
					if txt != "" {
						if onToken != nil {
							for _, w := range strings.SplitAfter(txt, " ") {
								onToken(w)
								time.Sleep(5 * time.Millisecond)
							}
						}
						return txt, true
					}
				}
			}
		}
	}

	// 3. ANTHROPIC CLAUDE CODE FRONTIER (claude-code-4.5-sonnet, claude-4.5-opus)
	if strings.HasPrefix(lower, "claude") {
		if key := GetProviderAPIKey("claude"); key != "" {
			candidates := []string{"claude-4-5-sonnet-latest", "claude-4-5-opus-latest", "claude-sonnet-4-5"}
			if strings.Contains(lower, "opus") {
				candidates = []string{"claude-4-5-opus-latest", "claude-opus-4-5", "claude-4-5-sonnet-latest"}
			}
			for _, apiModel := range candidates {
				reqBody, _ := json.Marshal(map[string]any{
					"model":      apiModel,
					"max_tokens": 4096,
					"system":     sysPromptWithBrain,
					"messages":   userMessages,
				})
				reqCtx, cancel := context.WithTimeout(ctx, 25*time.Second)
				req, err := http.NewRequestWithContext(reqCtx, http.MethodPost, "https://api.anthropic.com/v1/messages", bytes.NewReader(reqBody))
				if err != nil {
					cancel()
					continue
				}
				req.Header.Set("Content-Type", "application/json")
				req.Header.Set("x-api-key", key)
				req.Header.Set("anthropic-version", "2023-06-01")
				resp, rErr := http.DefaultClient.Do(req)
				if rErr != nil {
					cancel()
					continue
				}
				body, _ := io.ReadAll(resp.Body)
				_ = resp.Body.Close()
				cancel()
				if resp.StatusCode != http.StatusOK {
					continue
				}
				var parsed struct {
					Content []struct {
						Text string `json:"text"`
					} `json:"content"`
				}
				if json.Unmarshal(body, &parsed) == nil && len(parsed.Content) > 0 {
					txt := strings.TrimSpace(parsed.Content[0].Text)
					if txt != "" {
						if onToken != nil {
							for _, w := range strings.SplitAfter(txt, " ") {
								onToken(w)
								time.Sleep(5 * time.Millisecond)
							}
						}
						return txt, true
					}
				}
			}
		}
	}

	return "", false
}
