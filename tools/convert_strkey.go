package main

import (
	"encoding/hex"
	"fmt"
	"os"

	"github.com/stellar/go/strkey"
)

func main() {
	if len(os.Args) < 2 {
		fmt.Println("Usage: convert_strkey <strkey>")
		os.Exit(1)
	}

	contractStrkey := os.Args[1]

	// Decode contract strkey (starts with C)
	contractBytes, err := strkey.Decode(strkey.VersionByteContract, contractStrkey)
	if err != nil {
		fmt.Printf("Error decoding strkey: %v\n", err)
		os.Exit(1)
	}

	// Print as hex
	fmt.Printf("%s\n", hex.EncodeToString(contractBytes))
}
