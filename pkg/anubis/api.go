package anubis

import (
	"github.com/Groennesby/anubis-agent-core/internal/duat"
	"github.com/Groennesby/anubis-agent-core/internal/executor"
	"github.com/Groennesby/anubis-agent-core/internal/judgement"
	"github.com/Groennesby/anubis-agent-core/internal/memory"
	"github.com/Groennesby/anubis-agent-core/internal/tomb"
)

type BicameralEngine = executor.BicameralEngine
type ConversationTurn = executor.ConversationTurn
type TraceEvent = executor.TraceEvent
type Verdict = judgement.Verdict
type ScalesOfMaat = judgement.ScalesOfMaat
type BicameralVault = memory.BicameralVault
type RecallResult = memory.RecallResult
type SynapsePage = memory.SynapsePage
type TombSeal = tomb.TombSeal
type DuatDaemon = duat.DuatDaemon

const FeatherWeight = judgement.FeatherWeight

var AvailableModels = executor.AvailableModels

func NewBicameralEngine(workspaceDir string) (*BicameralEngine, error) {
	return executor.NewBicameralEngine(workspaceDir)
}
