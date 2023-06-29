package matrix

import (
	"fmt"

	_ "github.com/mattn/go-sqlite3"

	"maunium.net/go/mautrix/crypto"
	"maunium.net/go/mautrix/util/dbutil"
)

func isBadEncryptError(err error) bool {
	return err != crypto.SessionExpired && err != crypto.SessionNotShared && err != crypto.NoGroupSession
}

func (c *ClientWrapper) initCrypto() error {
	var err error

	//creates a new db on the provided path
	db, err := dbutil.NewWithDialect(c.config.DataDir, "sqlite3")
	if err != nil {
		return err
	}

	log := c.client.Log.With().Str("component", "crypto").Logger()
	accID := fmt.Sprintf("%s/%s", c.config.UserID.String(), c.config.DeviceID)
	cryptoStore := crypto.NewSQLCryptoStore(db, dbutil.ZeroLogger(log.With().Str("db_section", "matrix_state").Logger()), accID, c.config.DeviceID, []byte("thesis client"))

	//this flow is if we do not use the gomuks/config package
	/*if c.client.Store == nil {
		c.client.Store = cryptoStore
	} else if _, isMemory := c.client.Store.(*mautrix.MemorySyncStore); isMemory {
		c.client.Store = cryptoStore
	}
	err = cryptoStore.DB.Upgrade()
	if err != nil {
		return fmt.Errorf("failed to upgrade crypto state store: %w", err)
	}*/

	crypt := crypto.NewOlmMachine(c.client, &log, cryptoStore, c.config.Rooms)
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

func (c *ClientWrapper) cryptoOnLogin() {
	sqlStore, ok := c.crypto.CryptoStore.(*crypto.SQLCryptoStore)
	if !ok {
		return
	}
	sqlStore.DeviceID = c.config.DeviceID
	sqlStore.AccountID = fmt.Sprintf("%s/%s", c.config.UserID.String(), c.config.DeviceID)
}
