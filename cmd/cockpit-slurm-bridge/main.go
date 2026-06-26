package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"os"

	"github.com/lingweicai/cockpit-slurm/cmd/internal/provider"
)

func main() {
	var printNames bool
	flag.BoolVar(&printNames, "print-names", true, "print account names from sacctmgr")
	flag.BoolVar(&printNames, "p", true, "print account names from sacctmgr (shorthand)")
	flag.Parse()

	if !printNames {
		return
	}

	accounts, err := provider.NewSacctmgrAccountProvider().ListAccounts(context.Background())
	if err != nil {
		log.Printf("list accounts: %v", err)
		os.Exit(1)
	}

	for _, account := range accounts {
		fmt.Println(account.Name)
	}
}
