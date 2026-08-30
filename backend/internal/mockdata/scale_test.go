package mockdata

import (
	"context"
	"os"
	"runtime"
	"strconv"
	"sync"
	"testing"
	"time"

	"entitymatcher/matcher"
)

func TestScaleMatching(t *testing.T) {
	if os.Getenv("SCALE_TEST") != "1" {
		t.Skip("Skipping scale test. Set SCALE_TEST=1")
	}

	scaleNStr := os.Getenv("SCALE_N")
	scaleN := 100000
	if scaleNStr != "" {
		if val, err := strconv.Atoi(scaleNStr); err == nil {
			scaleN = val
		}
	}

	const batchID = "scale-test-batch"
	var m1, m2, m3, m4 runtime.MemStats

	// 1. Before Generation
	runtime.GC()
	runtime.ReadMemStats(&m1)
	genStart := time.Now()

	// 2. Generation
	sources, dests, _, _ := GenerateBigMockDataset(scaleN)
	genDuration := time.Since(genStart)
	runtime.ReadMemStats(&m2)

	// 3. Blocking Index Build
	blockStart := time.Now()
	_ = matcher.NewBlockingIndex(dests)
	blockDuration := time.Since(blockStart)
	runtime.ReadMemStats(&m3)

	// 4. Execute Job
	engine := matcher.NewMatchEngine(matcher.DefaultConfig())

	var peakHeap uint64
	stopMonitor := make(chan struct{})
	var wgMonitor sync.WaitGroup
	wgMonitor.Add(1)
	go func() {
		defer wgMonitor.Done()
		ticker := time.NewTicker(250 * time.Millisecond)
		defer ticker.Stop()
		for {
			select {
			case <-ticker.C:
				var m runtime.MemStats
				runtime.ReadMemStats(&m)
				if m.HeapAlloc > peakHeap {
					peakHeap = m.HeapAlloc
				}
			case <-stopMonitor:
				return
			}
		}
	}()

	execStart := time.Now()
	results, progress := engine.ExecuteJob(context.Background(), batchID, sources, dests, nil)
	execDuration := time.Since(execStart)

	close(stopMonitor)
	wgMonitor.Wait()
	runtime.ReadMemStats(&m4)

	// Calculations
	totalSources := len(sources)
	totalRows := len(results)
	throughput := float64(totalSources) / execDuration.Seconds()
	rowsPerSrc := float64(totalRows) / float64(totalSources)

	// Bytes per row = (HeapAlloc after ExecuteJob - HeapAlloc after generation) / number of result rows
	bytesPerRow := 0.0
	if totalRows > 0 {
		heapUsedByResults := int64(m4.HeapAlloc) - int64(m2.HeapAlloc)
		if heapUsedByResults > 0 {
			bytesPerRow = float64(heapUsedByResults) / float64(totalRows)
		}
	}

	cfg := matcher.DefaultConfig()

	t.Logf("\n=== Scale Matching Report (SCALE_N=%d) ===", scaleN)
	t.Logf("Config: WorkerCount=%d, MaxCandidatesPerSrc=%d, MaxAlternativesPerSource=%d",
		cfg.WorkerCount, cfg.MaxCandidatesPerSrc, cfg.MaxAlternativesPerSource)
	t.Logf("Thresholds: AutoMatch=%.2f, Review=%.2f, Margin=%.2f, ExactMatchFloor=%.2f",
		cfg.AutoMatchThreshold, cfg.ReviewThreshold, cfg.MarginThreshold, cfg.ExactMatchFloor)
	t.Logf("")
	t.Logf("GENERATION & DATA")
	t.Logf("  Generation Time:     %v", genDuration)
	t.Logf("  Actual Src Records:  %d", totalSources)
	t.Logf("  Actual Dst Records:  %d", len(dests))
	t.Logf("")
	t.Logf("BLOCKING INDEX")
	t.Logf("  Build Time:          %v", blockDuration)
	t.Logf("")
	t.Logf("EXECUTION")
	t.Logf("  ExecuteJob Time:     %v", execDuration)
	t.Logf("  Throughput:          %.2f sources/sec", throughput)
	t.Logf("")
	t.Logf("RESULTS")
	t.Logf("  Total Result Rows:   %d", totalRows)
	t.Logf("  Rows per Source:     %.2f", rowsPerSrc)
	t.Logf("  Auto-Matched:        %d", progress.AutoMatched)
	t.Logf("  Review-Needed:       %d", progress.ReviewNeeded)
	t.Logf("  No-Match:            %d", progress.NoMatchCount)
	t.Logf("")
	t.Logf("MEMORY (MiB)")
	t.Logf("  Before Generation:   HeapAlloc=%.2f, HeapSys=%.2f, Sys=%.2f, NumGC=%d",
		float64(m1.HeapAlloc)/1024/1024, float64(m1.HeapSys)/1024/1024, float64(m1.Sys)/1024/1024, m1.NumGC)
	t.Logf("  After Generation:    HeapAlloc=%.2f, HeapSys=%.2f, Sys=%.2f, NumGC=%d",
		float64(m2.HeapAlloc)/1024/1024, float64(m2.HeapSys)/1024/1024, float64(m2.Sys)/1024/1024, m2.NumGC)
	t.Logf("  After Blocking Idx:  HeapAlloc=%.2f, HeapSys=%.2f, Sys=%.2f, NumGC=%d",
		float64(m3.HeapAlloc)/1024/1024, float64(m3.HeapSys)/1024/1024, float64(m3.Sys)/1024/1024, m3.NumGC)
	t.Logf("  After ExecuteJob:    HeapAlloc=%.2f, HeapSys=%.2f, Sys=%.2f, NumGC=%d",
		float64(m4.HeapAlloc)/1024/1024, float64(m4.HeapSys)/1024/1024, float64(m4.Sys)/1024/1024, m4.NumGC)
	t.Logf("  Peak Heap Alloc:     %.2f MiB (during ExecuteJob)", float64(peakHeap)/1024/1024)
	t.Logf("")
	t.Logf("COST ANALYSIS")
	t.Logf("  Bytes per Result Row: %.2f bytes", bytesPerRow)
	t.Logf("==============================================")
}

func TestScaleSweep(t *testing.T) {
	if os.Getenv("SCALE_TEST") != "1" {
		t.Skip("Skipping scale sweep. Set SCALE_TEST=1")
	}

	sizes := []int{10000, 25000, 50000, 100000}
	const batchID = "sweep-batch"

	type row struct {
		size       int
		blockTime  time.Duration
		matchTime  time.Duration
		throughput float64
		peakHeapMB float64
	}
	var results []row

	for _, n := range sizes {
		runtime.GC()

		// Generation
		sources, dests, _, _ := GenerateBigMockDataset(n)

		// Blocking
		blockStart := time.Now()
		_ = matcher.NewBlockingIndex(dests)
		blockDur := time.Since(blockStart)

		// Matching
		engine := matcher.NewMatchEngine(matcher.DefaultConfig())

		var peakHeap uint64
		stopMonitor := make(chan struct{})
		var wgMonitor sync.WaitGroup
		wgMonitor.Add(1)
		go func() {
			defer wgMonitor.Done()
			ticker := time.NewTicker(250 * time.Millisecond)
			defer ticker.Stop()
			for {
				select {
				case <-ticker.C:
					var m runtime.MemStats
					runtime.ReadMemStats(&m)
					if m.HeapAlloc > peakHeap {
						peakHeap = m.HeapAlloc
					}
				case <-stopMonitor:
					return
				}
			}
		}()

		matchStart := time.Now()
		_, _ = engine.ExecuteJob(context.Background(), batchID, sources, dests, nil)
		matchDur := time.Since(matchStart)

		close(stopMonitor)
		wgMonitor.Wait()

		throughput := float64(len(sources)) / matchDur.Seconds()

		results = append(results, row{
			size:       n,
			blockTime:  blockDur,
			matchTime:  matchDur,
			throughput: throughput,
			peakHeapMB: float64(peakHeap) / 1024 / 1024,
		})

		// Cleanup for next iteration
		sources = nil
		dests = nil
		runtime.GC()
	}

	// Report Table
	t.Logf("\n=== Scale Sweep Report ===")
	t.Logf("%-10s | %-15s | %-15s | %-15s | %-15s",
		"Size(N)", "Block Time", "Match Time", "Throughput", "Peak Heap MiB")
	t.Logf("-----------|-----------------|-----------------|-----------------|----------------")
	for _, r := range results {
		t.Logf("%-10d | %-15v | %-15v | %-15.2f | %-15.2f",
			r.size, r.blockTime, r.matchTime, r.throughput, r.peakHeapMB)
	}
	t.Logf("===========================")
}
