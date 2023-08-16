package matrix

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"thesgo/matrix/mxevents"
	"time"

	"github.com/rs/zerolog"
	"maunium.net/go/mautrix/crypto"
	"maunium.net/go/mautrix/crypto/olm"
	"maunium.net/go/mautrix/event"
	"maunium.net/go/mautrix/id"
)

//Contains auxiliary functions to aid offline communcation, by emulating some of the cryptographic principles of matrix
//already provided by maunium.net/go SDK

// Auxiliary function to encrypt megolm session materials with Olm for offline communication if needed
func (c *ClientWrapper) encryptOlm(idKey id.Curve25519, fingerprintKey id.Ed25519, session *crypto.OlmSession, user id.UserID, evtType event.Type, content event.Content) *event.EncryptedEventContent {
	evt := &crypto.DecryptedOlmEvent{
		Sender:        c.client.UserID,
		SenderDevice:  c.client.DeviceID,
		Keys:          crypto.OlmEventKeys{Ed25519: c.crypto.GetAccount().SigningKey()},
		Recipient:     user,
		RecipientKeys: crypto.OlmEventKeys{Ed25519: fingerprintKey},
		Type:          evtType,
		Content:       content,
	}
	plaintext, err := json.Marshal(evt)
	if err != nil {
		panic(err)
	}

	c.logger.Debug().
		Str("recipient_identity_key", idKey.String()).
		Str("olm_session_id", session.ID().String()).
		Str("olm_session_description", session.Describe()).
		Msg("Encrypting olm message")
	msgType, ciphertext := session.Encrypt(plaintext)
	err = c.crypto.CryptoStore.UpdateSession(idKey, session)
	if err != nil {
		c.logger.Error().Err(err).Msg("Failed to update olm session in crypto store after encrypting")
	}
	return &event.EncryptedEventContent{
		Algorithm: id.AlgorithmOlmV1,
		SenderKey: c.crypto.GetAccount().IdentityKey(),
		OlmCiphertext: event.OlmCiphertexts{
			idKey: {
				Type: msgType,
				Body: string(ciphertext),
			},
		},
	}
}

func (c *ClientWrapper) decryptOlm(ctx context.Context, evt *event.Event) (*crypto.DecryptedOlmEvent, error) {
	content, ok := evt.Content.Parsed.(*event.EncryptedEventContent)
	if !ok {
		return nil, crypto.IncorrectEncryptedContentType
	} else if content.Algorithm != id.AlgorithmOlmV1 {
		return nil, crypto.UnsupportedAlgorithm
	}
	ownContent, ok := content.OlmCiphertext[c.crypto.GetAccount().IdentityKey()]
	if !ok {
		return nil, crypto.NotEncryptedForMe
	}
	decrypted, err := c.decryptAndParseOlmCiphertext(ctx, evt.Sender, content.SenderKey, ownContent.Type, ownContent.Body)
	if err != nil {
		return nil, err
	}
	decrypted.Source = evt
	return decrypted, nil
}

func (c *ClientWrapper) decryptAndParseOlmCiphertext(ctx context.Context, sender id.UserID, senderKey id.SenderKey, olmType id.OlmMsgType, ciphertext string) (*crypto.DecryptedOlmEvent, error) {
	if olmType != id.OlmMsgTypePreKey && olmType != id.OlmMsgTypeMsg {
		return nil, crypto.UnsupportedOlmMessageType
	}

	plaintext, err := c.tryDecryptOlmCiphertext(ctx, sender, senderKey, olmType, ciphertext)
	if err != nil {
		return nil, err
	}

	var olmEvt crypto.DecryptedOlmEvent
	err = json.Unmarshal(plaintext, &olmEvt)
	if err != nil {
		return nil, fmt.Errorf("failed to parse olm payload: %w", err)
	}
	if sender != olmEvt.Sender {
		return nil, crypto.SenderMismatch
	} else if c.client.UserID != olmEvt.Recipient {
		return nil, crypto.RecipientMismatch
	} else if c.crypto.GetAccount().SigningKey() != olmEvt.RecipientKeys.Ed25519 {
		return nil, crypto.RecipientKeyMismatch
	}

	err = olmEvt.Content.ParseRaw(olmEvt.Type)
	if err != nil && !errors.Is(err, event.ErrUnsupportedContentType) {
		return nil, fmt.Errorf("failed to parse content of olm payload event: %w", err)
	}

	olmEvt.SenderKey = senderKey

	return &olmEvt, nil
}

func (c *ClientWrapper) tryDecryptOlmCiphertext(ctx context.Context, sender id.UserID, senderKey id.SenderKey, olmType id.OlmMsgType, ciphertext string) ([]byte, error) {
	/*log := *zerolog.Ctx(ctx)
	endTimeTrace := mach.timeTrace(ctx, "waiting for olm lock", 5*time.Second)
	mach.olmLock.Lock()
	endTimeTrace()
	defer mach.olmLock.Unlock()*/

	plaintext, err := c.tryDecryptOlmCiphertextWithExistingSession(ctx, senderKey, olmType, ciphertext)
	if err != nil {
		if err == crypto.DecryptionFailedWithMatchingSession {
			c.logger.Warn().Msg("Found matching session, but decryption failed")
			//go mach.unwedgeDevice(log, sender, senderKey)
		}
		return nil, fmt.Errorf("failed to decrypt olm event: %w", err)
	}

	if plaintext != nil {
		// Decryption successful
		return plaintext, nil
	}

	/* won't need this
	// Decryption failed with every known session or no known sessions, let's try to create a new session.
	//
	// New sessions can only be created if it's a prekey message, we can't decrypt the message
	// if it isn't one at this point in time anymore, so return early.
	if olmType != id.OlmMsgTypePreKey {
		go mach.unwedgeDevice(log, sender, senderKey)
		return nil, crypto.DecryptionFailedForNormalMessage
	}

	/*log.Trace().Msg("Trying to create inbound session")
	endTimeTrace = mach.timeTrace(ctx, "creating inbound olm session", time.Second)
	session, err := c.crypto.createInboundSession(ctx, senderKey, ciphertext)
	endTimeTrace()
	if err != nil {
		go mach.unwedgeDevice(log, sender, senderKey)
		return nil, fmt.Errorf("failed to create new session from prekey message: %w", err)
	}
	log = log.With().Str("new_olm_session_id", session.ID().String()).Logger()
	log.Debug().
		Str("olm_session_description", session.Describe()).
		Msg("Created inbound olm session")
	ctx = log.WithContext(ctx)

	endTimeTrace = mach.timeTrace(ctx, "decrypting prekey olm message", time.Second)
	plaintext, err = session.Decrypt(ciphertext, olmType)
	endTimeTrace()
	if err != nil {
		go mach.unwedgeDevice(log, sender, senderKey)
		return nil, fmt.Errorf("failed to decrypt olm event with session created from prekey message: %w", err)
	}

	err = c.crypto.CryptoStore.UpdateSession(senderKey, session)
	endTimeTrace()
	if err != nil {
		log.Warn().Err(err).Msg("Failed to update new olm session in crypto store after decrypting")
	}
	return plaintext, nil*/
	return nil, nil
}

func (c *ClientWrapper) tryDecryptOlmCiphertextWithExistingSession(ctx context.Context, senderKey id.SenderKey, olmType id.OlmMsgType, ciphertext string) ([]byte, error) {
	log := *zerolog.Ctx(ctx)
	//endTimeTrace := mach.timeTrace(ctx, "getting sessions with sender key", time.Second)
	sessions, err := c.crypto.CryptoStore.GetSessions(senderKey)
	//endTimeTrace()
	if err != nil {
		return nil, fmt.Errorf("failed to get session for %s: %w", senderKey, err)
	}

	for _, session := range sessions {
		log := log.With().Str("olm_session_id", session.ID().String()).Logger()
		//ctx := log.WithContext(ctx)
		if olmType == id.OlmMsgTypePreKey {
			//endTimeTrace = mach.timeTrace(ctx, "checking if prekey olm message matches session", time.Second)
			matches, err := session.Internal.MatchesInboundSession(ciphertext)
			//endTimeTrace()
			if err != nil {
				return nil, fmt.Errorf("failed to check if ciphertext matches inbound session: %w", err)
			} else if !matches {
				continue
			}
		}
		log.Debug().Str("session_description", session.Describe()).Msg("Trying to decrypt olm message")
		//endTimeTrace = mach.timeTrace(ctx, "decrypting olm message", time.Second)
		plaintext, err := session.Decrypt(ciphertext, olmType)
		//endTimeTrace()
		if err != nil {
			if olmType == id.OlmMsgTypePreKey {
				return nil, crypto.DecryptionFailedWithMatchingSession
			}
		} else {
			//endTimeTrace = mach.timeTrace(ctx, "updating session in database", time.Second)
			err = c.crypto.CryptoStore.UpdateSession(senderKey, session)
			//endTimeTrace()
			if err != nil {
				log.Warn().Err(err).Msg("Failed to update olm session in crypto store after decrypting")
			}
			log.Debug().Msg("Decrypted olm message")
			return plaintext, nil
		}
	}
	return nil, nil
}

func (c *ClientWrapper) receiveRoomKey(ctx context.Context, evt *crypto.DecryptedOlmEvent, content *event.RoomKeyEventContent) {
	log := zerolog.Ctx(ctx).With().
		Str("algorithm", string(content.Algorithm)).
		Str("session_id", content.SessionID.String()).
		Str("room_id", content.RoomID.String()).
		Logger()
	if content.Algorithm != id.AlgorithmMegolmV1 || evt.Keys.Ed25519 == "" {
		log.Debug().Msg("Ignoring weird room key")
		return
	}

	config := c.crypto.StateStore.GetEncryptionEvent(content.RoomID)
	var maxAge time.Duration
	var maxMessages int
	if config != nil {
		maxAge = time.Duration(config.RotationPeriodMillis) * time.Millisecond
		if maxAge == 0 {
			maxAge = 7 * 24 * time.Hour
		}
		maxMessages = config.RotationPeriodMessages
		if maxMessages == 0 {
			maxMessages = 100
		}
	}
	if content.MaxAge != 0 {
		maxAge = time.Duration(content.MaxAge) * time.Millisecond
	}
	if content.MaxMessages != 0 {
		maxMessages = content.MaxMessages
	}
	if c.crypto.DeletePreviousKeysOnReceive && !content.IsScheduled {
		log.Debug().Msg("Redacting previous megolm sessions from sender in room")
		sessionIDs, err := c.crypto.CryptoStore.RedactGroupSessions(content.RoomID, evt.SenderKey, "received new key from device")
		if err != nil {
			log.Err(err).Msg("Failed to redact previous megolm sessions")
		} else {
			log.Info().
				Strs("session_ids", stringifyArray(sessionIDs)).
				Msg("Redacted previous megolm sessions")
		}
	}
	c.createGroupSession(ctx, evt.SenderKey, evt.Keys.Ed25519, content.RoomID, content.SessionID, content.SessionKey, maxAge, maxMessages, content.IsScheduled)
}

func (c *ClientWrapper) createGroupSession(ctx context.Context, senderKey id.SenderKey, signingKey id.Ed25519, roomID id.RoomID, sessionID id.SessionID, sessionKey string, maxAge time.Duration, maxMessages int, isScheduled bool) {
	log := zerolog.Ctx(ctx)
	igs, err := crypto.NewInboundGroupSession(senderKey, signingKey, roomID, sessionKey, maxAge, maxMessages, isScheduled)
	if err != nil {
		log.Error().Err(err).Msg("Failed to create inbound group session")
		return
	} else if igs.ID() != sessionID {
		log.Warn().
			Str("expected_session_id", sessionID.String()).
			Str("actual_session_id", igs.ID().String()).
			Msg("Mismatched session ID while creating inbound group session")
		return
	}
	err = c.crypto.CryptoStore.PutGroupSession(roomID, senderKey, sessionID, igs)
	if err != nil {
		log.Error().Err(err).Str("session_id", sessionID.String()).Msg("Failed to store new inbound group session")
		return
	}
	//mach.markSessionReceived(sessionID)
	log.Debug().
		Str("session_id", sessionID.String()).
		Str("sender_key", senderKey.String()).
		Str("max_age", maxAge.String()).
		Int("max_messages", maxMessages).
		Bool("is_scheduled", isScheduled).
		Msg("Received inbound group session")
}

func (c *ClientWrapper) buildKeyRequest(roomID id.RoomID, senderKey id.SenderKey, sessionID id.SessionID, users map[id.UserID][]id.DeviceID) *mxevents.Event {
	requestCont := &event.Content{
		Parsed: &event.RoomKeyRequestEventContent{
			Action: event.KeyRequestActionRequest,
			Body: event.RequestedKeyInfo{
				Algorithm: id.AlgorithmMegolmV1,
				RoomID:    roomID,
				SenderKey: senderKey,
				SessionID: sessionID,
			},
			RequestID:          c.client.TxnID(),
			RequestingDeviceID: c.client.DeviceID,
		},
	}

	requestEvent := &mxevents.Event{
		Event: &event.Event{
			Type:    event.ToDeviceRoomKeyRequest,
			Content: event.Content{Parsed: requestCont},
		},
	}

	return requestEvent
}

func (c *ClientWrapper) parseKeyRequestEvent(content *event.RoomKeyRequestEventContent) (event.Content, error) {
	igs, err := c.crypto.CryptoStore.GetGroupSession(content.Body.RoomID, content.Body.SenderKey, content.Body.SessionID)
	if err != nil {
		if errors.Is(err, crypto.ErrGroupSessionWithheld) {
			c.logger.Debug().Err(err).Msg("Requested group session not available")
		} else {
			c.logger.Error().Err(err).Msg("Failed to get group session to forward")
		}
		return event.Content{}, err
	} else if igs == nil {
		c.logger.Error().Msg("Didn't find group session to forward")
		return event.Content{}, fmt.Errorf("didn't find group session to forward")
	}
	exportedKey, _ := igs.Internal.Export(igs.Internal.FirstKnownIndex())
	forwardedRoomKey := event.Content{
		Parsed: &event.ForwardedRoomKeyEventContent{
			RoomKeyEventContent: event.RoomKeyEventContent{
				Algorithm:  id.AlgorithmMegolmV1,
				RoomID:     igs.RoomID,
				SessionID:  igs.ID(),
				SessionKey: string(exportedKey),
			},
			SenderKey:          content.Body.SenderKey,
			ForwardingKeyChain: igs.ForwardingChains,
			SenderClaimedKey:   igs.SigningKey,
		},
	}
	return forwardedRoomKey, nil
}

func (c *ClientWrapper) importForwardedRoomKey(ctx context.Context, evt *crypto.DecryptedOlmEvent, content *event.ForwardedRoomKeyEventContent) bool {
	log := zerolog.Ctx(ctx).With().
		Str("session_id", content.SessionID.String()).
		Str("room_id", content.RoomID.String()).
		Logger()
	if content.Algorithm != id.AlgorithmMegolmV1 || evt.Keys.Ed25519 == "" {
		log.Debug().
			Str("algorithm", string(content.Algorithm)).
			Msg("Ignoring weird forwarded room key")
		return false
	}

	igsInternal, err := olm.InboundGroupSessionImport([]byte(content.SessionKey))
	if err != nil {
		log.Error().Err(err).Msg("Failed to import inbound group session")
		return false
	} else if igsInternal.ID() != content.SessionID {
		log.Warn().
			Str("actual_session_id", igsInternal.ID().String()).
			Msg("Mismatched session ID while creating inbound group session from forward")
		return false
	}
	config := c.crypto.StateStore.GetEncryptionEvent(content.RoomID)
	var maxAge time.Duration
	var maxMessages int
	if config != nil {
		maxAge = time.Duration(config.RotationPeriodMillis) * time.Millisecond
		maxMessages = config.RotationPeriodMessages
	}
	if content.MaxAge != 0 {
		maxAge = time.Duration(content.MaxAge) * time.Millisecond
	}
	if content.MaxMessages != 0 {
		maxMessages = content.MaxMessages
	}

	//major problem here: when device is online again, there can be session id mismatches since we cannot change id
	//outside of maunium.net/crypto package -> however, we do check if the internal ID is different from content ID on line 360
	igs := &crypto.InboundGroupSession{
		Internal:         *igsInternal,
		SigningKey:       evt.Keys.Ed25519,
		SenderKey:        content.SenderKey,
		RoomID:           content.RoomID,
		ForwardingChains: append(content.ForwardingKeyChain, evt.SenderKey.String()),
		//id:               content.SessionID

		ReceivedAt:  time.Now().UTC(),
		MaxAge:      maxAge.Milliseconds(),
		MaxMessages: maxMessages,
		IsScheduled: content.IsScheduled,
	}
	err = c.crypto.CryptoStore.PutGroupSession(content.RoomID, content.SenderKey, content.SessionID, igs)
	if err != nil {
		log.Error().Err(err).Msg("Failed to store new inbound group session")
		return false
	}

	//mach.markSessionReceived(content.SessionID) -> cannot call this, may not be necessary for offline, but may impact
	//consistency when device comes back online
	log.Debug().Msg("Received forwarded inbound group session")
	return true
}

func stringifyArray[T ~string](arr []T) []string {
	strs := make([]string, len(arr))
	for i, v := range arr {
		strs[i] = string(v)
	}
	return strs
}
