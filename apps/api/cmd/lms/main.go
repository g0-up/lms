// Lệnh lms: API HTTP, migration, seed, worker outbox và healthcheck trong một binary.
package main

import "os"

func main() {
	if err := Execute(); err != nil {
		os.Exit(1)
	}
}
