package main

import (
	"bufio"
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

// Logging

func logMessage(prefix string, v ...interface{}) {
	fmt.Fprintln(os.Stderr, prefix, fmt.Sprint(v...))
}

func logInfo(v ...interface{}) {
	logMessage("Info:", v...)
}

func logError(v ...interface{}) {
	logMessage("Error:", v...)
}

func logFatal(v ...interface{}) {
	logError(v...)
	os.Exit(1)
}

func logHelp(v ...interface{}) {
	logError(v...)
	Help()
	os.Exit(1)
}

// Input

var scanner = bufio.NewScanner(os.Stdin)

func scanLine() string {
	if !scanner.Scan() {
		logFatal("failed to scan line")
	}
	return scanner.Text()
}

// Constants

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
  config          Get this utility config path
  directory       Get %s directory path
  help            Print this help
`

// Program

type Config struct {
	configFilePath string
	WalletsDir     string            `json:"wallet-directory"`
	CurrentWallet  string            `json:"current-wallet"`
	Wallets        map[string]string `json:"wallets"`
}

func getConfigFilePath() (string, error) {
	configFilePath, err := xdg.SearchConfigFile(configFilePath)
	if err != nil {
		var err2 error
		configFilePath, err2 = xdg.ConfigFile(configFilePath)
		if err2 != nil {
			// something is horribly wrong (directories can't be created, probably)
			logFatal(err2)
		}
	}
	return configFilePath, err
}

func getConfig() (Config, error) {
	var config Config

	var err error
	config.configFilePath, err = getConfigFilePath()
	if err != nil {
		return config, err
	}

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

func addWallet(config *Config, walletDirName string, walletExists bool, changeCurrentWallet bool) error {
	walletName := walletDirName

	if walletName == currentWalletDirName {
		for {
			fmt.Printf("Enter name for the \"%s\" wallet (can't be \"%s\"; leave empty to keep the current): ", walletDirName, currentWalletDirName)
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
		err := addWallet(config, walletDirName, true, true)
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
	if config.Wallets[walletName] == "" {
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
	err = addWallet(config, walletDirName, exists, false)
	if err != nil {
		return err
	}
	// If user creates a wallet, he probably wants to use it right away, so it is a good idea to
	// switch to it
	return Switch(config, walletDirName)
}

func Forget(config *Config, walletName string) error {
	if config.Wallets[walletName] == "" {
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

func Status(config *Config) {
	fmt.Println(getCount(len(config.Wallets)) + ":")
	for name, description := range config.Wallets {
		if name == config.CurrentWallet {
			description += " (current)"
		}
		fmt.Printf("%s: %s\n", name, description)
	}
}

func Config_(config *Config) {
	fmt.Println(config.configFilePath)
}

func Directory(config *Config) {
	fmt.Println(config.WalletsDir)
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
			default:
				logHelp("unknown subcommand")
			}
		} else {
			switch subcommand {
			case "init":
				config.configFilePath, err = getConfigFilePath()
				if err != nil {
					logFatal(err)
				}
				config.WalletsDir = getWalletsDir()
				wrapSubcommand(Init(&config))
			case "status":
				loadConfig()
				Status(&config)
			case "config":
				config.configFilePath, err = getConfigFilePath()
				if err != nil {
					logFatal(err)
				}
				Config_(&config)
			case "directory":
				config.WalletsDir = getWalletsDir()
				Directory(&config)
			case "help":
				Help()
			case "switch", "edit", "add", "forget":
				logHelp("no argument")
			default:
				logHelp("unknown subcommand")
			}
		}
	} else {
		logHelp("no subcommand specified")
	}
}
