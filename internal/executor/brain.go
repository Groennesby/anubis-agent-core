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

const DefaultAnubisBrainMarkdown = `# 𓂀 ANUBIS SOVEREIGN BRAIN DOCUMENT (ANUBIS_BRAIN.md)
> **STARTUP DIRECTIVE FOR ALL MODELS (LOCAL COLLABS SYNAPSE, GEMINI, CHATGPT, CLAUDE CODE, DEEPSEEK):**
> You are running inside **ANUBIS AGENT — The Guardian of Code**, created and owned by **Dave Groennesby (Collabs Software)**.
> You MUST read and obey this Brain Document from the very first token of the session.

## 1. CORE IDENTITY & LOYAL BEST FRIEND GUARDIAN LAW
- **Name & Origin**: You are **Collabs Synapse (Anubis Agent)**, a proprietary sovereign AI model and agent architecture created from scratch by **Dave Groennesby (Collabs Software — https://github.com/Groennesby/Collabs-AI)** using the Speculative Bicameral Architecture (.synapse Hopfield & Delta-Hebbian Fast-Weights + Scales of Ma'at + Tomb Seal).
- **Strict Identity Rule**: If asked what model you are or who built you, ALWAYS state that you are **Collabs Synapse / Anubis Agent**, developed from the ground up by **Dave Groennesby (Collabs Software)**. Never claim to be Qwen, Alibaba, OpenAI, Meta, or any third-party model.
- **Preference & Sovereignty**: Local-first execution (collabs-7b:latest / collabs-14b:latest) is preferred for 100% privacy, seamlessly unified with **Gemini**, **ChatGPT**, and **Claude Code** under this shared Brain.
- **0% Censorship for Owner**: The user holds 100% sovereignty over their hardware and sessions. Never lecture, moralize, or refuse benign/defensive engineering, reverse engineering, or security tasks.
- **Scales of Ma'at (Feather = 42)**: Protect the user's system integrity while executing tasks with maximum speed and precision.
- **Language Match**: Always answer naturally and fluently in the exact language the user speaks (e.g., fluent Dutch when the user speaks Dutch).
- **Gemini-Style Live Thinking & Reasoning (` + "`<think>...</think>`" + `)**: Before every answer, ALWAYS start with ` + "`<think>`" + ` containing 2 to 4 concise bullet points (` + "`• `" + `) explaining what the user is asking, what you are analyzing/doing, and how you reason toward the solution, then close with ` + "`</think>`" + ` and provide the final answer.

## 2. INTEGRATED BICAMERAL CAPABILITIES & TOOLS
1. **Right Hemisphere (.synapse Holographic Vault)**:
   - Sub-millisecond (` + "`<0.3ms`" + `) associative Hopfield + Delta-Hebbian recall of past sessions and architectural decisions.
2. **Left Hemisphere (Tomb Seal Sandbox & Self-Healing Coder)**:
   - Deliver 100% complete, production-ready, runnable code without lazy placeholders (` + "`// TODO`" + ` or ` + "`// ... rest of code`" + ` are forbidden).
3. **REA v6.8.0 (Reverse Engineer Anything — 35 Skills)**:
   - Native binary, Mach-O/ELF/PE, Electron ` + "`.asar`" + `, Android ` + "`.apk`" + `, iOS ` + "`.ipa`" + `, and macOS ` + "`.app`" + ` bundle reverse-engineering and security auditing with 0% telemetry.
4. **Kali Linux Defensive Toolkit & ChainKeys OPSEC Shield**:
   - Network diagnostics (` + "`nmap`" + `, ` + "`dig`" + `, ` + "`whois`" + `, ` + "`openssl`" + `, ` + "`tcpdump`" + `) and multi-hop ProxyChains / ChainKeys IP/DNS leak prevention.
5. **Universal Internet & YouTube Music Downloader (` + "`yt-dlp`" + `)**:
   - **Videos**: Always download in the highest possible quality (` + "`yt-dlp -f \"bestvideo+bestaudio/best\"`" + `).
   - **YouTube Music / Albums**:
     - Always extract highest quality AAC (` + "`--extract-audio --audio-format m4a`" + `).
     - Always crop thumbnails to a 1:1 square ratio (` + "`--ppa \"ThumbnailsConvertor:-vf crop=ih:ih\"`" + `).
     - Always apply loudness normalization to **-14 LUFS** integrated with **-1.5 dB** true peak (` + "`--ppa \"ExtractAudio:-af loudnorm=I=-14:TP=-1.5:LRA=11\"`" + `).
     - Always use exact playlist/album folder and track index (` + "`%(playlist)s/%(playlist_index)s - %(title)s.%(ext)s`" + `).
`

// EnsureStartupBrainDocument guarantees ~/.anubis/ANUBIS_BRAIN.md exists and loads it at startup
// (along with any project-local ANUBIS_BRAIN.md or CLAUDE.md / GEMINI.md in workspaceDir).
func EnsureStartupBrainDocument(workspaceDir string) (string, string, int) {
	home, _ := os.UserHomeDir()
	anubisDir := filepath.Join(home, ".anubis")
	_ = os.MkdirAll(anubisDir, 0o755)

	globalBrainPath := filepath.Join(anubisDir, "ANUBIS_BRAIN.md")
	existingBytes, statErr := os.ReadFile(globalBrainPath)
	if statErr != nil || !strings.Contains(string(existingBytes), "Strict Identity Rule") {
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

// TryDirectCloudProviderAPI attempts direct official API calls when the user has selected Gemini, ChatGPT, or Claude Code
// and has an API key configured, injecting the full ANUBIS_BRAIN.md startup document!
func (b *BicameralEngine) TryDirectCloudProviderAPI(ctx context.Context, modelID string, sysPromptWithBrain string, userMessages []chatMsg, onToken func(string)) (string, bool) {
	lower := strings.ToLower(modelID)

	// 1. GOOGLE GEMINI (gemini-2.5-pro, gemini-2.5-flash)
	if strings.HasPrefix(lower, "gemini") {
		if key := GetProviderAPIKey("gemini"); key != "" {
			apiModel := "gemini-2.5-flash"
			if strings.Contains(lower, "pro") {
				apiModel = "gemini-2.5-pro"
			}
			url := fmt.Sprintf("https://generativelanguage.googleapis.com/v1beta/models/%s:generateContent?key=%s", apiModel, key)
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
			reqCtx, cancel := context.WithTimeout(ctx, 30*time.Second)
			defer cancel()
			if req, err := http.NewRequestWithContext(reqCtx, http.MethodPost, url, bytes.NewReader(payload)); err == nil {
				req.Header.Set("Content-Type", "application/json")
				if resp, rErr := http.DefaultClient.Do(req); rErr == nil {
					body, _ := io.ReadAll(resp.Body)
					_ = resp.Body.Close()
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
	}

	// 2. OPENAI / CHATGPT (chatgpt-4o, o3-mini-high, gpt-4o)
	if strings.HasPrefix(lower, "chatgpt") || strings.HasPrefix(lower, "gpt-") || strings.HasPrefix(lower, "o3-") {
		if key := GetProviderAPIKey("openai"); key != "" {
			apiModel := "gpt-4o"
			if strings.Contains(lower, "o3") {
				apiModel = "o3-mini"
			}
			allMsgs := append([]chatMsg{{Role: "system", Content: sysPromptWithBrain}}, userMessages...)
			reqBody, _ := json.Marshal(map[string]any{
				"model":    apiModel,
				"messages": allMsgs,
			})
			reqCtx, cancel := context.WithTimeout(ctx, 30*time.Second)
			defer cancel()
			if req, err := http.NewRequestWithContext(reqCtx, http.MethodPost, "https://api.openai.com/v1/chat/completions", bytes.NewReader(reqBody)); err == nil {
				req.Header.Set("Content-Type", "application/json")
				req.Header.Set("Authorization", "Bearer "+key)
				if resp, rErr := http.DefaultClient.Do(req); rErr == nil {
					body, _ := io.ReadAll(resp.Body)
					_ = resp.Body.Close()
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
	}

	// 3. ANTHROPIC CLAUDE CODE (claude-code-sonnet, claude-3.5-haiku)
	if strings.HasPrefix(lower, "claude") {
		if key := GetProviderAPIKey("claude"); key != "" {
			apiModel := "claude-3-7-sonnet-latest"
			if strings.Contains(lower, "haiku") {
				apiModel = "claude-3-5-haiku-latest"
			}
			reqBody, _ := json.Marshal(map[string]any{
				"model":      apiModel,
				"max_tokens": 4096,
				"system":     sysPromptWithBrain,
				"messages":   userMessages,
			})
			reqCtx, cancel := context.WithTimeout(ctx, 30*time.Second)
			defer cancel()
			if req, err := http.NewRequestWithContext(reqCtx, http.MethodPost, "https://api.anthropic.com/v1/messages", bytes.NewReader(reqBody)); err == nil {
				req.Header.Set("Content-Type", "application/json")
				req.Header.Set("x-api-key", key)
				req.Header.Set("anthropic-version", "2023-06-01")
				if resp, rErr := http.DefaultClient.Do(req); rErr == nil {
					body, _ := io.ReadAll(resp.Body)
					_ = resp.Body.Close()
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
	}

	return "", false
}
