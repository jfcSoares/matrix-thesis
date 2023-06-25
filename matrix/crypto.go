package matrix

import (
	"fmt"

	_ "github.com/mattn/go-sqlite3"

	"maunium.net/go/mautrix/crypto"
)

func isBadEncryptError(err error) bool {
	return err != crypto.SessionExpired && err != crypto.SessionNotShared && err != crypto.NoGroupSession
}

func (c *ClientWrapper) initCrypto() error {
	var cryptoStore crypto.Store
	var err error

	//for now, only save crypto data in memory
	memStore := crypto.NewMemoryStore(saveStoreData)
	cryptoStore = memStore

	crypt := crypto.NewOlmMachine(c.client, &c.logger, cryptoStore, c.rooms)
	c.crypto = crypt
	err = c.crypto.Load()
	if err != nil {
		return fmt.Errorf("failed to create olm machine: %w", err)
	}
	return nil

}

func saveStoreData() error {
	return nil
}
