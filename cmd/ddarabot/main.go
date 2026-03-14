package main

import (
	"fmt"
	"os"
)

var version = "dev"

func main() {
	if len(os.Args) > 1 && os.Args[1] == "version" {
		fmt.Printf("ddarabot %s\n", version)
		return
	}
	fmt.Println("ddarabot: not yet implemented")
}
