package main

import (
	"errors"
	"os"
	"time"
	"bytes"
	"fmt"

	"github.com/ethereum/go-ethereum/cmd/utils"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/rawdb"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/log"
	"github.com/urfave/cli/v2"
)

func dumpBinaryState(ctx *cli.Context) error {
	stack, _ := makeConfigNode(ctx)
	defer stack.Close()

	db := utils.MakeChainDatabase(ctx, stack, true)
	defer db.Close()

	triedb := utils.MakeTrieDatabase(ctx, stack, db, false, true, false)
	defer triedb.Close()

	var (
		onlyEoa = ctx.Bool("onlyEOA")
		//onlySafeOwners = ctx.Bool("onlyEOA")
		start    = time.Now()
		logged   = time.Now()
		written  uint64
		accounts uint64
		head     = rawdb.ReadHeadHeader(db)
		root     = head.Root
	)

	stateIt, err := utils.NewStateIterator(triedb, db, root)
	if err != nil {
		return err
	}
	accIt, err := stateIt.AccountIterator(root, (common.Hash{}))
	if err != nil {
		return err
	}
	defer accIt.Release()

	outfile := ctx.Args().First()
	if outfile == "" {
		return errors.New("an outfile prefix must be specified")
	}
	date := time.Unix(int64(head.Time), 0)
	specifier := "all"
	if onlyEoa {
		specifier = "_onlyEoa_"
	}
	fileName := fmt.Sprintf("%v_%s_%d_%s.dat",
		outfile, specifier, head.Number, date.UTC().String())
	out, err := os.Create(fileName)
	if err != nil {
		return err
	}
	defer out.Close()

	log.Info("Snapshot dumping started", "root", root, "outfile", fileName)

	for accIt.Next() {
		if onlyEoa {
			// We need to verify that this is an EOA. Either it has no code,
			// OR it has a delegation
			account, err := types.FullAccount(accIt.Account())
			if err != nil {
				return err
			}
			if !bytes.Equal(account.CodeHash, types.EmptyCodeHash.Bytes()) {
				code := rawdb.ReadCode(db, common.BytesToHash(account.CodeHash))
				if _, delegated := types.ParseDelegation(code); delegated {
					// This is a delegated EOA
					out.Write(accIt.Hash().Bytes())
					written++
				}
			} else {
				// This is an EOA
				out.Write(accIt.Hash().Bytes())
				written++
			}
		} else {
			out.Write(accIt.Hash().Bytes())
			written++
		}
		accounts++
		if time.Since(logged) > 8*time.Second {
			log.Info("Snapshot dumping in progress", "at", accIt.Hash(),
				"accounts", accounts, "written", written,
				"elapsed", common.PrettyDuration(time.Since(start)))
			logged = time.Now()
		}
	}
	log.Info("Snapshot dumping complete", "accounts", accounts,
		"elapsed", common.PrettyDuration(time.Since(start)))
	return nil
}
