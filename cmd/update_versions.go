package main

import (
    "fmt"
    "os"

    "github.com/ssotops/gitspace/tools"
)

func main() {
    fmt.Println("Starting version update process...")

    if err := tools.UpdateVersions(); err != nil {
        fmt.Printf("Error updating versions: %v\n", err)
        os.Exit(1)
    }

    fmt.Println("Version update process completed successfully.")
}
