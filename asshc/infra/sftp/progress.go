package sftp

import (
	"fmt"
	"time"
)

type ProgressInfo struct {
	Index    int
	Total    int
	FileName string
	Progress float64
	Rate     string
	ETA      string
	Bytes    int64
	TotalBytes int64
}

type TransferProgress func(info ProgressInfo)

func NewProgressCallback(total int) TransferProgress {
	return func(info ProgressInfo) {
		if total <= 0 {
			total = 1
		}
		prefix := fmt.Sprintf("(%*d/%d)", len(fmt.Sprintf("%d", total)), info.Index, total)

		barLen := 30
		bar := make([]byte, barLen)
		progress := info.Progress
		if progress > 1 {
			progress = 1
		}
		if progress < 0 {
			progress = 0
		}
		filled := int(progress * float64(barLen))
		for i := 0; i < filled; i++ {
			bar[i] = '='
		}
		if filled < barLen {
			bar[filled] = '.'
			for i := filled + 1; i < barLen; i++ {
				bar[i] = ' '
			}
		}

		percent := int(progress * 100)
		rate := info.Rate
		if rate == "" {
			rate = "0B/s"
		}
		eta := info.ETA
		if eta == "" {
			eta = "ETA ?"
		}

		name := info.FileName
		if len(name) > 20 {
			name = name[:17] + "..."
		}
		name = fmt.Sprintf("[%-20s]", name)

		barStr := string(bar)
		fmt.Printf("\r%s %s %s %3d%% %10s %8s", prefix, name, barStr, percent, rate, eta)
		if progress >= 1 {
			fmt.Println()
		}
	}
}

func formatRate(bytesPerSec float64) string {
	if bytesPerSec < 1024 {
		return fmt.Sprintf("%.0fB/s", bytesPerSec)
	}
	if bytesPerSec < 1024*1024 {
		return fmt.Sprintf("%.1fKB/s", bytesPerSec/1024)
	}
	if bytesPerSec < 1024*1024*1024 {
		return fmt.Sprintf("%.1fMB/s", bytesPerSec/1024/1024)
	}
	return fmt.Sprintf("%.1fGB/s", bytesPerSec/1024/1024/1024)
}

func formatETA(seconds int) string {
	if seconds <= 0 {
		return "0s"
	}
	if seconds < 60 {
		return fmt.Sprintf("%ds", seconds)
	}
	if seconds < 3600 {
		return fmt.Sprintf("%dm", seconds/60)
	}
	return fmt.Sprintf("%dh", seconds/3600)
}

type ProgressTracker struct {
	startTime    time.Time
	lastBytes    int64
	lastTime     time.Time
	windowSize   int64
}

func NewProgressTracker() *ProgressTracker {
	now := time.Now()
	return &ProgressTracker{
		startTime:  now,
		lastBytes:   0,
		lastTime:    now,
		windowSize:  5 * 1024 * 1024,
	}
}

func (pt *ProgressTracker) Update(currentBytes int64) (rate string, eta string) {
	now := time.Now()
	elapsed := now.Sub(pt.startTime).Seconds()
	if elapsed <= 0 {
		elapsed = 1
	}

	totalRate := float64(currentBytes) / elapsed

	windowBytes := currentBytes - pt.lastBytes
	windowElapsed := now.Sub(pt.lastTime).Seconds()
	var instantRate float64
	if windowElapsed > 0 {
		instantRate = float64(windowBytes) / windowElapsed
		pt.lastBytes = currentBytes
		pt.lastTime = now
	}

	avgRate := totalRate
	if instantRate > 0 && instantRate < totalRate*2 {
		avgRate = (totalRate + instantRate) / 2
	}

	remaining := float64(pt.windowSize) - float64(currentBytes)
	var etaSeconds int
	if avgRate > 0 {
		etaSeconds = int(remaining / avgRate)
	}

	return formatRate(avgRate), formatETA(etaSeconds)
}