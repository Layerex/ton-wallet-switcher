package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"io/ioutil"
	"os"
	"path/filepath"
	"strings"

	"github.com/adrg/xdg"
)

const walletsDirName = "TON Wallet"
const currentWalletDirName = "data"
const configFilePath = "ton-wallet-switcher/ton-wallet-switcher.json"

const helpMessage = `Usage: %s [COMMAND] [WALLET]

An utility for managing multiple TON Wallet wallets

Commands:
  init            Initialize: find all wallets and ask user to describe them
  status          List wallets
  switch [WALLET] Switch to another wallet
  edit [WALLET]   Edit wallet name and description
  add [WALLET]    Add an existing wallet directory or create a new one
  forget [WALLET] Forget about wallet
  remove [WALLET] Forget about wallet and remove its directory
  config          Get this utility config path
  directory       Get %s directory path
  help            Print this help
`

type Config struct {
	configFilePath string
	WalletsDir     string            `json:"wallet-directory"`
	CurrentWallet  string            `json:"current-wallet"`
	Wallets        map[string]string `json:"wallets"`
}

func getConfigFilePath() string {
	configFilePath, err := xdg.ConfigFile(configFilePath)
	if err != nil {
		logFatal(err)
	}
	return configFilePath
}

func getConfig() (Config, error) {
	var config Config

	var err error
	config.configFilePath = getConfigFilePath()

	rawConfig, err := ioutil.ReadFile(config.configFilePath)
	if err != nil {
		return config, err
	}

	err = json.Unmarshal(rawConfig, &config)
	if err != nil {
		return config, err
	}

	return config, nil
}

func writeConfig(config *Config) error {
	encodedConfig, err := json.MarshalIndent(config, "", "\t")
	if err != nil {
		return err
	}

	err = ioutil.WriteFile(config.configFilePath, encodedConfig, fs.FileMode(0660))
	if err != nil {
		return err
	}

	return nil
}

func getWalletsDir() string {
	walletsDir, err := xdg.SearchDataFile(walletsDirName)
	if err != nil {
		logError(err)
		for {
			fmt.Printf("Enter the path to \"%s\" directory: ", walletsDirName)
			walletsDir = scanLine()

			walletDirInfo, err := os.Stat(walletsDir)
			if err == nil && walletDirInfo.IsDir() {
				break
			}

			logError("path invalid")
		}
	}
	return walletsDir
}

func isWalletDir(walletsDir string, walletDir string) bool {
	saltFilePath := filepath.Join(walletsDir, walletDir, "salt")
	saltFileInfo, err := os.Stat(saltFilePath)
	return err == nil && !saltFileInfo.IsDir()
}

func getWallets(walletsDir string) ([]string, error) {
	var wallets []string
	walletEntries, err := os.ReadDir(walletsDir)
	if err != nil {
		return wallets, err
	}
	wallets = make([]string, 0, len(walletEntries))
	for _, walletEntry := range walletEntries {
		if isWalletDir(walletsDir, walletEntry.Name()) {
			wallets = append(wallets, walletEntry.Name())
		}
	}
	return wallets, nil
}

func addWallet(config *Config, walletDirName string, changeCurrentWallet bool) error {
	walletName := walletDirName

	if walletName == currentWalletDirName {
		for {
			if walletDirName == currentWalletDirName {
				fmt.Printf("Name the \"%s\" wallet (name can't be the current one): ", walletDirName)
			} else {
				fmt.Printf("Name the \"%s\" wallet (name can't be \"%s\"; leave empty to keep the current one): ", walletDirName, currentWalletDirName)
			}
			walletName = scanLine()
			if walletName == "" {
				walletName = walletDirName
			}
			if walletName != currentWalletDirName {
				break
			}
			logError("wallet name invalid")
		}
	}

	if changeCurrentWallet && walletDirName == currentWalletDirName {
		config.CurrentWallet = walletName
	}

	fmt.Printf("Enter description for the \"%s\" wallet: ", walletName)
	config.Wallets[walletName] = scanLine()
	return nil
}

func getCount(count int) string {
	var noun string
	if count == 1 {
		noun = "wallet"
	} else {
		noun = "wallets"
	}
	return fmt.Sprintf("%d %s", count, noun)
}

func switchToFirstWallet(config *Config) error {
	for name := range config.Wallets {
		return Switch(config, name)
	}
	return nil
}

func Init(config *Config) error {
	wallets, err := getWallets(config.WalletsDir)
	if err != nil {
		return err
	}

	logInfo(getCount(len(wallets)), " located")

	config.Wallets = make(map[string]string)
	config.CurrentWallet = ""
	for _, walletDirName := range wallets {
		err := addWallet(config, walletDirName, true)
		if err != nil {
			return err
		}
	}

	if config.CurrentWallet == "" {
		err = switchToFirstWallet(config)
		if err != nil {
			return err
		}
	}

	return nil
}

func Switch(config *Config, walletName string) error {
	if config.CurrentWallet == walletName {
		return fmt.Errorf("already switched to wallet \"%s\"", walletName)
	}
	_, ok := config.Wallets[walletName]
	if !ok {
		return fmt.Errorf("no wallet \"%s\" present", walletName)
	}
	if config.CurrentWallet != "" {
		err := os.Rename(currentWalletDirName, config.CurrentWallet)
		if err != nil {
			return err
		}
	}
	err := os.Rename(walletName, currentWalletDirName)
	if err != nil && !errors.Is(err, os.ErrNotExist) {
		return err
	}
	config.CurrentWallet = walletName
	return nil
}

func Edit(config *Config, walletName string) error {
	_, ok := config.Wallets[walletName]
	if !ok {
		return fmt.Errorf("no wallet \"%s\" present", walletName)
	}
	var newWalletName string
	for {
		fmt.Printf("Enter new name for the \"%s\" wallet (can't be \"%s\"; leave empty to keep the name): ", walletName, currentWalletDirName)
		newWalletName = scanLine()
		if newWalletName == "" {
			newWalletName = walletName
		}
		if newWalletName != currentWalletDirName {
			break
		}
		logError("wallet name invalid")
	}

	fmt.Printf("Enter description for the \"%s\" (\"%s\") wallet (leave empty to keep the description): ", walletName, newWalletName)
	newWalletDescription := scanLine()
	if newWalletDescription == "" {
		newWalletDescription = config.Wallets[walletName]
	}

	if config.CurrentWallet == walletName {
		config.CurrentWallet = newWalletName
	} else if newWalletName != walletName {
		err := os.Rename(walletName, newWalletName)
		if err != nil {
			return err
		}
	}
	config.Wallets[newWalletName] = newWalletDescription
	if walletName != newWalletName {
		delete(config.Wallets, walletName)
	}
	return nil
}

func getRelativeWalletDirectory(walletDirName string) string {
	return filepath.Join(walletsDirName, walletDirName)
}

func Add(config *Config, walletDirName string) error {
	walletDirInfo, err := os.Stat(walletDirName)
	exists := !errors.Is(err, os.ErrNotExist)
	if exists {
		if !walletDirInfo.IsDir() {
			return fmt.Errorf("\"%s\" is not a directory", getRelativeWalletDirectory(walletDirName))
		}
		if !isWalletDir(config.WalletsDir, walletDirName) {
			return fmt.Errorf("\"%s\" is not a wallet directory", getRelativeWalletDirectory(walletDirName))
		}
	}
	err = addWallet(config, walletDirName, false)
	if err != nil {
		return err
	}
	// If user creates a wallet, he probably wants to use it right away, so it is a good idea to
	// switch to it
	return Switch(config, walletDirName)
}

func Forget(config *Config, walletName string) error {
	_, ok := config.Wallets[walletName]
	if !ok {
		return fmt.Errorf("no wallet \"%s\" present", walletName)
	}
	if config.CurrentWallet == walletName {
		err := os.Rename(currentWalletDirName, walletName)
		if err != nil && !errors.Is(err, os.ErrNotExist) {
			return err
		}
		delete(config.Wallets, walletName)
		config.CurrentWallet = ""
		err = switchToFirstWallet(config)
		if err != nil {
			return err
		}
	} else {
		delete(config.Wallets, walletName)
	}
	return nil
}

func Remove(config *Config, walletName string) error {
	_, ok := config.Wallets[walletName]
	if !ok {
		return fmt.Errorf("no wallet \"%s\" present", walletName)
	}
	fmt.Printf("Do you really want to remove the \"%s\" wallet? Type \"yes\" to confirm: ", walletName)
	confirmation := scanLine()
	if confirmation == "yes" {
		err := Forget(config, walletName)
		if err != nil {
			return err
		}
		err = os.RemoveAll(walletName)
		if err != nil && !errors.Is(err, os.ErrNotExist) {
			return err
		}
		return nil
	}
	return fmt.Errorf("operation aborted")
}

func Status(config *Config) {
	fmt.Println(getCount(len(config.Wallets)) + ":")
	for name, description := range config.Wallets {
		if name == config.CurrentWallet {
			description += " (current)"
		}
		fmt.Printf("%s: %s\n", name, description)
	}
}

func Help() {
	fmt.Printf(helpMessage, os.Args[0], walletsDirName)
}

func main() {
	var config Config
	var err error

	wrapSubcommand := func(err error) {
		if err != nil {
			logFatal(err)
		}
		err = writeConfig(&config)
		if err != nil {
			logFatal(err)
		}
	}

	loadConfig := func() {
		config, err = getConfig()
		if config.WalletsDir == "" {
			config.WalletsDir = getWalletsDir()
		}
		err2 := os.Chdir(config.WalletsDir)
		if err2 != nil {
			logFatal(err2)
		}
		if err != nil {
			logError(err)
			logInfo("failed to read config file, performing initialization")
			wrapSubcommand(Init(&config))
			os.Exit(0)
		}
	}

	if len(os.Args) > 1 {
		subcommand := os.Args[1]
		if len(os.Args) > 2 {
			argument := strings.Join(os.Args[2:], " ")
			switch subcommand {
			case "switch":
				loadConfig()
				wrapSubcommand(Switch(&config, argument))
			case "edit":
				loadConfig()
				wrapSubcommand(Edit(&config, argument))
			case "add":
				loadConfig()
				wrapSubcommand(Add(&config, argument))
			case "forget":
				loadConfig()
				wrapSubcommand(Forget(&config, argument))
			case "remove":
				loadConfig()
				wrapSubcommand(Remove(&config, argument))
			default:
				logHelp("unknown subcommand")
			}
		} else {
			switch subcommand {
			case "init":
				config.configFilePath = getConfigFilePath()
				config.WalletsDir = getWalletsDir()
				wrapSubcommand(Init(&config))
			case "status":
				loadConfig()
				Status(&config)
			case "config":
				fmt.Println(getConfigFilePath())
			case "directory":
				fmt.Println(getWalletsDir())
			case "help":
				Help()
			case "switch", "edit", "add", "forget", "remove":
				logHelp("no argument")
			default:
				logHelp("unknown subcommand")
			}
		}
	} else {
		logHelp("no subcommand specified")
	}
}
