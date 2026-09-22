// Package languagebudget bounds transient parser reservations across all ES languages.
package languagebudget

import "golang.org/x/sync/semaphore"

const MemoryLimit int64 = 128 << 20

var Memory = semaphore.NewWeighted(MemoryLimit)
