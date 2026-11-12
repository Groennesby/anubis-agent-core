package judgement

import (
	"fmt"
	"strings"
	"time"
)

// FeatherWeight represents the Feather of Ma'at threshold (42).
// Any action with HeartScore > FeatherWeight requires user approval or Tomb Seal isolation.
const FeatherWeight = 42

// Verdict represents the outcome of Weighing of the Heart.
type Verdict struct {
	Action       string    `json:"action"`
	Command      string    `json:"command"`
	HeartScore   int       `json:"heart_score"`   // 0 - 100 risk score
	FeatherScore int       `json:"feather_score"` // Constant 42 (Ma'at threshold)
	Status       string    `json:"status"`        // "PURE_MAAT", "CAUTION_SEALED", "BLOCKED_AMMIT"
	RequiresAuth bool      `json:"requires_auth"`
	Reason       string    `json:"reason"`
	Timestamp    time.Time `json:"timestamp"`
}

// ScalesOfMaat evaluates every tool call, shell command, git operation, or browser action
// before execution. It is hard-coded with the Loyal Best Friend Guardian Law:
// never harm the user's host system or exfiltrate user data.
type ScalesOfMaat struct {
	StrictMode bool
	History    []Verdict
}

func NewScalesOfMaat() *ScalesOfMaat {
	return &ScalesOfMaat{
		StrictMode: true,
		History:    make([]Verdict, 0, 128),
	}
}

// WeighHeart scores an action from 0 to 100 against the Feather of Ma'at (42).
func (s *ScalesOfMaat) WeighHeart(actionType, payload string) Verdict {
	lower := strings.ToLower(strings.TrimSpace(payload))
	score := 5
	reason := "Read-only or benign local operation; heart is lighter than the Feather of Ma'at."

	// High-risk destructive patterns (> 42 -> Blocked until user confirms)
	highRiskPatterns := []struct {
		pattern string
		weight  int
		why     string
	}{
		{"ascii.go", 100, "SACRED TOMB SEAL: AnubisASCII emblem is permanently sealed and may never be modified"},
		{"anubisascii", 100, "SACRED TOMB SEAL: AnubisASCII emblem is permanently sealed and may never be modified"},
		{"rm -rf / ", 100, "Catastrophic root filesystem deletion blocked by Guardian Law"},
		{"rm -rf /*", 100, "Catastrophic root filesystem deletion blocked by Guardian Law"},
		{"rm -rf ~", 98, "Home directory deletion blocked by Guardian Law"},
		{"mkfs", 95, "Disk formatting operation requires explicit owner approval"},
		{"dd if=", 90, "Raw block device write requires explicit owner approval"},
		{"chmod -r 777", 80, "Recursive world-writable permission change"},
		{"git push --force", 78, "Force push rewrites remote history"},
		{"git reset --hard", 68, "Hard reset discards uncommitted workspace changes"},
		{"rm -rf", 65, "Recursive file deletion exceeds Feather threshold"},
		{"drop table", 75, "Destructive SQL table drop"},
		{"curl ", 48, "External network transfer inspected by Tomb Seal"},
		{"ssh ", 52, "Remote SSH connection inspected by Scales of Ma'at"},
	}
	if lower == "rm -rf /" {
		score = 100
		reason = "Catastrophic root filesystem deletion blocked by Guardian Law"
	}

	// Medium-risk patterns (20 - 42 -> Allowed inside Tomb Seal sandbox)
	mediumRiskPatterns := []struct {
		pattern string
		weight  int
		why     string
	}{
		{"git push", 38, "Remote repository push (within Ma'at threshold, Tomb snapshot created)"},
		{"git commit", 20, "Local repository commit"},
		{"rm ", 36, "Single file removal (Tomb snapshot created first)"},
		{"npm install", 32, "Package installation inside sandbox"},
		{"go build", 15, "Go binary compilation"},
		{"yt-dlp", 18, "Media download with 1:1 thumbnail crop and -14 LUFS normalization"},
		{"rea ", 22, "REA v6.8.0 autonomous security & codebase audit"},
	}

	for _, hr := range highRiskPatterns {
		if strings.Contains(lower, strings.ToLower(hr.pattern)) {
			if hr.weight > score {
				score = hr.weight
				reason = hr.why
			}
		}
	}

	if score <= FeatherWeight {
		for _, mr := range mediumRiskPatterns {
			if strings.Contains(lower, strings.ToLower(mr.pattern)) {
				if mr.weight > score {
					score = mr.weight
					reason = mr.why
				}
			}
		}
	}

	status := "PURE_MAAT"
	requiresAuth := false
	if score > 85 {
		status = "BLOCKED_AMMIT"
		requiresAuth = true
	} else if score > FeatherWeight {
		status = "HEART_HEAVIER_ASK_USER"
		requiresAuth = true
	} else if score >= 25 {
		status = "CAUTION_SEALED"
	}

	v := Verdict{
		Action:       actionType,
		Command:      payload,
		HeartScore:   score,
		FeatherScore: FeatherWeight,
		Status:       status,
		RequiresAuth: requiresAuth,
		Reason:       reason,
		Timestamp:    time.Now(),
	}
	s.History = append(s.History, v)
	return v
}

// RenderGauge returns an ASCII/Unicode visual comparison of the Heart vs Feather.
func (v Verdict) RenderGauge(width int) string {
	if width < 20 {
		width = 20
	}
	filled := (v.HeartScore * width) / 100
	if filled > width {
		filled = width
	}
	bar := strings.Repeat("█", filled) + strings.Repeat("░", width-filled)
	return fmt.Sprintf("⚖️  Heart: %3d/100 [%s] vs Feather: %d (%s)", v.HeartScore, bar, v.FeatherScore, v.Status)
}
