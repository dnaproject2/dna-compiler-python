/*
 * Copyright (C) 2019 The dna Authors
 * This file is part of The dna library.
 *
 * The dna is free software: you can redistribute it and/or modify
 * it under the terms of the GNU Lesser General Public License as published by
 * the Free Software Foundation, either version 3 of the License, or
 * (at your option) any later version.
 *
 * The dna is distributed in the hope that it will be useful,
 * but WITHOUT ANY WARRANTY; without even the implied warranty of
 * MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
 * GNU Lesser General Public License for more details.
 *
 * You should have received a copy of the GNU Lesser General Public License
 * along with The dna.  If not, see <http://www.gnu.org/licenses/>.
 */

package main

import (
	"encoding/hex"
	"errors"
	"fmt"
	"io/ioutil"
	"math"
	"os"
	"runtime"
	"strings"
	"time"

	"github.com/dnaproject2/DNA/account"
	"github.com/dnaproject2/DNA/cmd"
	"github.com/dnaproject2/DNA/cmd/utils"
	common3 "github.com/dnaproject2/DNA/common"
	"github.com/dnaproject2/DNA/common/config"
	"github.com/dnaproject2/DNA/common/log"
	"github.com/dnaproject2/DNA/core/payload"
	"github.com/dnaproject2/DNA/core/store/ledgerstore"
	"github.com/dnaproject2/DNA/core/store/overlaydb"
	"github.com/dnaproject2/DNA/core/types"
	//utils2 "github.com/dnaproject2/DNA/core/utils"
	common2 "github.com/dnaproject2/DNA/http/base/common"
	"github.com/dnaproject2/DNA/smartcontract"
	"github.com/dnaproject2/DNA/smartcontract/event"
	"github.com/dnaproject2/DNA/smartcontract/service/neovm"
	"github.com/dnaproject2/DNA/smartcontract/storage"
	//neotype "github.com/dnaproject2/DNA/vm/neovm/types"
	"github.com/urfave/cli"
)

const (
	DEFAULT_BYTECODE    = "./test.avm.str"
	DEFAULT_LEDGER_PATH = "./Chain"
)

var (
	//nvm-tool setting
	NvmByteCodeFlag = cli.StringFlag{
		Name:  "bytecode,b",
		Usage: "smart contract bytecode.",
		Value: DEFAULT_BYTECODE,
	}
	LedgerPathFlag = cli.StringFlag{
		Name:  "ledger,l",
		Usage: "ledger path",
		Value: DEFAULT_LEDGER_PATH,
	}
	VMTypeFlag = cli.BoolFlag{
		Name:  "type,t",
		Usage: "type t",
	}
	ContractParamsFlag = cli.StringFlag{
		Name:  "param,p",
		Usage: "with param",
	}
	CallVmType bool = true
)

func setupAPP() *cli.App {
	app := cli.NewApp()
	app.Usage = "NeoVM CLI"
	app.Action = neovmCLI
	app.Version = config.Version
	app.Copyright = "Copyright in 2019 The dna Authors"
	app.Flags = []cli.Flag{
		NvmByteCodeFlag,
		LedgerPathFlag,
		VMTypeFlag,
		ContractParamsFlag,
	}
	app.Before = func(context *cli.Context) error {
		runtime.GOMAXPROCS(runtime.NumCPU())
		return nil
	}
	return app
}

func main() {
	if err := setupAPP().Run(os.Args); err != nil {
		cmd.PrintErrorMsg(err.Error())
		os.Exit(1)
	}
}

func neovmCLI(ctx *cli.Context) {
	// assert the vm type
	// create account
	owner := account.NewAccount("")

	// read nvm bytecode
	codeFile := ctx.String(utils.GetFlagName(NvmByteCodeFlag))
	log.Infof("test file %s", codeFile)
	codeStr, err := ioutil.ReadFile(codeFile)
	if err != nil {
		log.Errorf("open nvm code file failed: %s", err)
		panic(err)
	}
	code, err := hex.DecodeString(strings.TrimSpace(string(codeStr)))
	if err != nil {
		log.Errorf("failed to decode code from hex to binary: %s", err)
		panic(err)
	}

	paramsStr := ctx.String(utils.GetFlagName(ContractParamsFlag))
	params, err := utils.ParseParams(paramsStr)
	if err != nil {
		log.Errorf("praseParam err: %s", err)
	}

	var gaslimit uint64 = math.MaxUint64
	mtx := utils.NewDeployCodeTransaction(0, gaslimit, code, CallVmType, "test", "test", "test", "test", "test")
	d := mtx.Payload.(*payload.DeployCode)
	if d == nil {
		log.Errorf("failed to get smart contract deploy address")
		panic("generate deploy tx faild")
	}
	contractAddr := d.Address()

	// init ledger
	ledgerDir := ctx.String(utils.GetFlagName(LedgerPathFlag))
	dbPath := fmt.Sprintf("%s%s%s", ledgerDir, string(os.PathSeparator), ledgerstore.DBDirState)
	merklePath := fmt.Sprintf("%s%s%s", ledgerDir, string(os.PathSeparator), ledgerstore.MerkleTreeStorePath)
	stateStore, err := ledgerstore.NewStateStore(dbPath, merklePath, 0)
	if err != nil {
		log.Errorf("failed to create state store: %s", err)
		panic(err)
	}
	overlay := stateStore.NewOverlayDB()

	// deploy nvm byte code
	if err := executeDeployTx(stateStore, overlay, owner, mtx); err != nil {
		log.Errorf("failed to deploy smart contract: %s", err)
		panic(err)
	}
	log.Infof("deploy %s done, address = %s", codeFile, contractAddr.ToHexString())

	testResult, gas, err := executeMethodargs(CallVmType, contractAddr, params, stateStore, overlay, owner)
	if err != nil {
		log.Errorf("executeMethodargs: %s", err)
		panic(err)
	}
	log.Infof("Sum Gas: %d", gas)
	log.Infof("Result:	%s", testResult)
}

func executeMethodargs(vmType bool, contractAddr common3.Address, params []interface{}, stateStore *ledgerstore.StateStore, overlay *overlaydb.OverlayDB, user *account.Account) (string, uint64, error) {
	var mtx *types.MutableTransaction
	if CallVmType {
		// acctually sc.Gas will ignore this gaslimit(second arg) on this test. so pass zero is ok.
		mtxl, err := common2.NewNeovmInvokeTransaction(0, 0, contractAddr, params)
		if err != nil {
			return "", 0, fmt.Errorf("%s", err)
		}
		mtx = mtxl
	} else {
		return "", 0, errors.New("VM type error")
	}

	return executeInvokeTx(stateStore, overlay, user, mtx)
}

func executeDeployTx(store *ledgerstore.StateStore, overlay *overlaydb.OverlayDB, user *account.Account, mtx *types.MutableTransaction) error {
	cache := storage.NewCacheDB(overlay)

	if err := utils.SignTransaction(user, mtx); err != nil {
		return fmt.Errorf("sign deploy: %s", err)
	}
	tx, err := mtx.IntoImmutable()
	if err != nil {
		return fmt.Errorf("deploy tx immu: %s", err)
	}

	notify := &event.ExecuteNotify{TxHash: tx.Hash(), State: event.CONTRACT_STATE_FAIL}
	if err := store.HandleDeployTransaction(nil, overlay, cache, tx, nil, notify); err != nil {
		return fmt.Errorf("handle deploy tx: %s", err)
	}
	cache.Commit()
	return nil
}

func executeInvokeTx(store *ledgerstore.StateStore, overlay *overlaydb.OverlayDB, user *account.Account, mtx *types.MutableTransaction) (string, uint64, error) {
	if err := utils.SignTransaction(user, mtx); err != nil {
		return "", 0, fmt.Errorf("failed to sign tx: %s", err)
	}
	tx, err := mtx.IntoImmutable()
	if err != nil {
		return "", 0, fmt.Errorf("failed to invoke tx immu: %s", err)
	}

	cache := storage.NewCacheDB(overlay)
	config := &smartcontract.Config{
		Time:   uint32(time.Now().Unix()),
		Height: 1000000000,
		Tx:     tx,
	}
	invoke := tx.Payload.(*payload.InvokeCode)

	gasTable := make(map[string]uint64)
	neovm.GAS_TABLE.Range(func(k, value interface{}) bool {
		key := k.(string)
		val := value.(uint64)
		gasTable[key] = val

		return true
	})

	sc := smartcontract.SmartContract{
		Config:  config,
		Store:   nil,
		CacheDB: cache,
		Gas:     math.MaxUint64,
		//PreExec: true,
	}

	engine, err := sc.NewExecuteEngine(invoke.Code)
	if err != nil {
		return "", 0, fmt.Errorf("start exec engine failed: %s", err)
	}

	_, err = engine.Invoke()

	if err != nil {
		return "", 0, fmt.Errorf("preexec invoke failed: %s", err)
	}

	cache.Commit()

	return "", math.MaxUint64 - sc.Gas, nil
}
