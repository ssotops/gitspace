// cmd/update_versions/main.go
package main

import (
	"fmt"
	"os"

	"github.com/ssotspace/gitspace/tools"
)

func main() {
	if err := tools.UpdateVersions(); err != nil {
		fmt.Printf("Error updating versions: %v\n", err)
		os.Exit(1)
	}
	fmt.Println("Versions updated successfully")
}
