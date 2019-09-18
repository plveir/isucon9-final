package bencherror

import (
	"sync"

	"github.com/chibiegg/isucon9-final/bench/internal/config"
	"go.uber.org/zap"
)

var (
	InitializeErrs = NewBenchErrors()
	PreTestErrs    = NewBenchErrors()
	BenchmarkErrs  = NewBenchErrors()
)

type BenchErrors struct {
	mu sync.RWMutex

	Msgs []string

	criticalCnt    uint64
	applicationCnt uint64
	timeoutCnt     uint64
	temporaryCnt   uint64
}

func NewBenchErrors() *BenchErrors {
	return &BenchErrors{
		Msgs: []string{},
	}
}

// IsError は、エラーが発生したか否かを返します
func (errs *BenchErrors) IsError() bool {
	errs.mu.RLock()
	defer errs.mu.RUnlock()

	return len(errs.Msgs) > 0
}

// IsFailure は失格したか否かを返します
func (errs *BenchErrors) IsFailure() bool {
	errs.mu.RLock()
	defer errs.mu.RUnlock()

	if errs.criticalCnt > 0 || errs.applicationCnt >= 10 {
		return true
	}
	return false
}

func (errs *BenchErrors) Penalty() uint64 {
	errs.mu.RLock()
	defer errs.mu.RUnlock()

	lgr := zap.S()
	lgr.Infow("エラーカウンタ覧",
		"critical", errs.criticalCnt,
		"application", errs.applicationCnt,
		"timeout", errs.timeoutCnt,
		"temporary", errs.temporaryCnt,
	)

	penalty := config.ApplicationPenaltyWeight * errs.applicationCnt
	lgr.Infof("アプリのエラーによるペナルティ減算後: %d", penalty)

	trivialCnt := errs.timeoutCnt + errs.temporaryCnt
	if trivialCnt > config.TrivialPenaltyThreshold {
		lgr.Warn("タイムアウトや一時的なエラーが閾値を超えています")
		penalty += config.TrivialPenaltyWeight * (1 + (trivialCnt-config.TrivialPenaltyThreshold)/config.TrivialPenaltyPerCount)
		lgr.Infof("タイムアウトや一時的なエラーによるペナルティ減算後: %d", penalty)
	}

	return penalty
}

func (errs *BenchErrors) AddError(err error) {
	lgr := zap.S()

	errs.mu.Lock()
	defer errs.mu.Unlock()

	if err == nil {
		return
	}

	lgr.Warnf("エラーを追加: %+v", err)

	// エラーに応じたメッセージを追加し、カウンタをインクリメント
	if msg, code, ok := extractCode(err); ok {
		switch code {
		case errCritical:
			errs.Msgs = append(errs.Msgs, msg+" (critical error)")
			errs.criticalCnt++
		case errApplication:
			errs.Msgs = append(errs.Msgs, msg)
			errs.applicationCnt++
		case errTimeout:
			errs.Msgs = append(errs.Msgs, msg+" (タイムアウトしました)")
			errs.timeoutCnt++
		case errTemporary:
			errs.Msgs = append(errs.Msgs, msg+" (一時的なエラー)")
			errs.temporaryCnt++
		default:
			errs.Msgs = append(errs.Msgs, "運営に連絡してください")
			errs.criticalCnt++
		}
	}
}