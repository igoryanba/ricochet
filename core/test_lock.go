package main

import (
	"fmt"
	"log"
	"os"
	"path/filepath"
	"time"

	"github.com/gofrs/flock"
)

func main() {
	homeDir, _ := os.UserHomeDir()
	lockPath := filepath.Join(homeDir, ".ricochet", "test-bot.lock")
	os.MkdirAll(filepath.Dir(lockPath), 0755)

	name := "Instance 1"
	if len(os.Args) > 1 {
		name = os.Args[1]
	}

	f := flock.New(lockPath)
	fmt.Printf("[%s] Trying to lock %s...\n", name, lockPath)

	locked, err := f.TryLock()
	if err != nil {
		log.Fatal(err)
	}

	if !locked {
		fmt.Printf("[%s] Lock already held by another process.\n", name)
		return
	}

	fmt.Printf("[%s] Lock acquired! Sleeping for 10s...\n", name)
	time.Sleep(10 * time.Second)

	f.Unlock()
	fmt.Printf("[%s] Lock released.\n", name)
}
