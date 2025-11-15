package main

import (
	"fmt"
	"github.com/stellar/go/xdr"
)

func main() {
	temporary := xdr.ContractDataDurabilityTemporary
	persistent := xdr.ContractDataDurabilityPersistent

	fmt.Printf("Temporary: '%s' (length: %d)\n", temporary.String(), len(temporary.String()))
	fmt.Printf("Persistent: '%s' (length: %d)\n", persistent.String(), len(persistent.String()))
}
