package config

import (
	"os"
	"path/filepath"
)

const AppName = "minbit"

func AppDir() string {
	dir, _ := os.UserHomeDir()
	return filepath.Join(dir, AppName)
}

func WalletDir() string {
	return filepath.Join(AppDir(), "wallets")
}

func StoreDir() string {
	return filepath.Join(AppDir(), "store")
}
