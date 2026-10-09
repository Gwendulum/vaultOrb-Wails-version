package main

import (
	"context"
	"fmt"
	"strings"
	"sync"
	"time"
	"vault-wails/internal/db"
)

type redoActionType int

const (
	redoActionRegister = iota
	redoActionUpdate
	redoActionDelete
)

type DbRow struct {
	ID        int64  `json:"id"`
	Domain    string `json:"domain"`
	Username  string `json:"username"`
	Password  string `json:"password"`
	CreatedAt string `json:"created_at"`
	UpdatedAt string `json:"updated_at"`
}

type historyEntry struct {
	kind    redoActionType
	payload db.Entry
	isUndo  bool
}

type VaultService struct {
	dbQ        *db.Queries
	wordlist   []string
	dbPath     string
	isFirstRun bool

	mu          sync.RWMutex
	masterKey   []byte
	list        []db.Entry
	undoHistory []historyEntry
	redoHistory []historyEntry
}

// Constructor function to cleanly instantiate the service
func NewVaultService(dbQ *db.Queries, wordlist []string, dbPath string, isFirstRun bool) *VaultService {
	return &VaultService{
		dbQ:        dbQ,
		wordlist:   wordlist,
		dbPath:     dbPath,
		isFirstRun: isFirstRun,
	}
}

func (v *VaultService) Login(password string) error {
	ctx := context.Background()
	masterKey, err := unlockVault(ctx, v.dbQ, []byte(password))
	if err != nil {
		return err
	}
	v.mu.Lock()
	v.masterKey = masterKey
	v.mu.Unlock()

	return nil
}

func (v *VaultService) List() ([]DbRow, error) {
	ctx := context.Background()
	entries, err := listPassword(ctx, v.dbQ)
	if err != nil {
		return nil, err
	}
	var result []DbRow
	for _, entry := range entries {
		createdAt := formatTime(entry.CreatedAt)
		updatedAt := formatTime(entry.UpdatedAt)
		newEntry := DbRow{
			ID:        entry.ID,
			Domain:    entry.Domain,
			Username:  entry.Username,
			CreatedAt: createdAt,
			UpdatedAt: updatedAt,
		}
		result = append(result, newEntry)
	}

	v.mu.Lock()
	v.list = entries
	v.mu.Unlock()
	return result, nil
}

func formatTime(t time.Time) string {
	fields := strings.Fields(t.Local().Format("15:04:05 02-01-2006"))
	time := fields[0]
	dateField := fields[1]
	dateField = dateField[:7] + dateField[9:]
	var result string

	date := strings.Replace(dateField, "-", "/", 2)

	result = time[:5] + " - " + date

	return result
}

func (v *VaultService) Register(domain string, user string, password string) error {
	var domUserPass []string
	domUserPass = append(domUserPass, domain, user, password)
	ctx := context.Background()

	v.mu.RLock()
	key := v.masterKey
	v.mu.RUnlock()

	newEntry, err := registerPassword(ctx, v.dbQ, domUserPass, key)
	if err != nil {
		return err
	}
	newUndoEntry := historyEntry{
		kind:    redoActionDelete,
		payload: newEntry,
		isUndo:  true,
	}

	v.mu.Lock()
	v.undoHistory = append(v.undoHistory, newUndoEntry)
	v.mu.Unlock()
	return nil
}

func (v *VaultService) Get(domain string, user string) (string, error) {

	args := []string{domain, user}
	ctx := context.Background()
	password, err := getPassword(ctx, v.dbQ, args, v.masterKey)
	if err != nil {
		return "", err
	}

	return password, nil
}

func (v *VaultService) Delete(domain string, user string) error {
	var domUser []string
	domUser = append(domUser, domain, user)
	ctx := context.Background()

	deletedEntry, err := deletePassword(ctx, v.dbQ, domUser)
	if err != nil {
		return err
	}
	newUndoEntry := historyEntry{
		kind:    redoActionRegister,
		payload: deletedEntry,
		isUndo:  true,
	}

	v.mu.Lock()
	v.undoHistory = append(v.undoHistory, newUndoEntry)
	v.mu.Unlock()
	return nil
}

func (v *VaultService) Update(domain string, user string, password string) error {
	ctx := context.Background()
	args := []string{domain, user}
	oldEntry, err := getEntry(ctx, v.dbQ, args, v.masterKey)
	if err != nil {
		return err
	}
	_, err = updatePassword(ctx, v.dbQ, password, args, v.masterKey)
	if err != nil {
		return err
	}

	newUndoEntry := historyEntry{
		kind:    redoActionUpdate,
		payload: oldEntry,
		isUndo:  true,
	}

	v.mu.Lock()
	v.undoHistory = append(v.undoHistory, newUndoEntry)
	v.mu.Unlock()

	return nil

}

func (v *VaultService) Undo() error {
	entry, err := v.popUndo()
	if err != nil {
		return err
	}
	ctx := context.Background()
	switch entry.kind {
	case redoActionRegister:
		if err := v.handleRedoRegister(ctx, entry); err != nil {
			return err
		}
		return nil
	case redoActionDelete:
		if err := v.handleRedoDelete(ctx, entry); err != nil {
			return err
		}
		return nil

	case redoActionUpdate:
		if err := v.handleRedoUpdate(ctx, entry); err != nil {
			return err
		}
		return nil
	}

	return nil
}

func (v *VaultService) Redo() error {
	entry, err := v.popRedo()
	if err != nil {
		return err
	}
	ctx := context.Background()
	switch entry.kind {
	case redoActionRegister:
		if err := v.handleRedoRegister(ctx, entry); err != nil {
			return err
		}
		return nil
	case redoActionDelete:
		if err := v.handleRedoDelete(ctx, entry); err != nil {
			return err
		}
		return nil

	case redoActionUpdate:
		if err := v.handleRedoUpdate(ctx, entry); err != nil {
			return err
		}
		return nil
	}

	return nil
}

// helper functions
func (v *VaultService) popUndo() (historyEntry, error) {
	n := len(v.undoHistory)
	if n <= 0 {
		return historyEntry{}, fmt.Errorf("nothing to undo")
	}
	entry := v.undoHistory[n-1]

	v.undoHistory = v.undoHistory[:len(v.undoHistory)-1]
	if !entry.isUndo {
		return historyEntry{}, fmt.Errorf("redo entry on undo stack")
	}

	return entry, nil
}

func (v *VaultService) popRedo() (historyEntry, error) {
	n := len(v.redoHistory)
	if n <= 0 {
		return historyEntry{}, fmt.Errorf("nothing to redo")
	}
	entry := v.redoHistory[n-1]

	v.redoHistory = v.redoHistory[:len(v.redoHistory)-1]
	if entry.isUndo {
		return historyEntry{}, fmt.Errorf("undo entry on redo stack")
	}
	return entry, nil
}

// handler functions
func (v *VaultService) handleRedoRegister(ctx context.Context, entry historyEntry) error {
	restoredDbEntry, err := v.dbQ.RestoreEntry(ctx, db.RestoreEntryParams{
		ID:                entry.payload.ID,
		Domain:            entry.payload.Domain,
		Username:          entry.payload.Username,
		EncryptedPassword: entry.payload.EncryptedPassword,
		CreatedAt:         entry.payload.CreatedAt,
		UpdatedAt:         entry.payload.UpdatedAt,
	})
	if err != nil {
		return err
	}
	newHistoryEntry := historyEntry{
		kind:    redoActionDelete,
		payload: restoredDbEntry,
	}

	v.mu.Lock()
	if entry.isUndo {
		newHistoryEntry.isUndo = false
		v.redoHistory = append(v.redoHistory, newHistoryEntry)
	} else {
		newHistoryEntry.isUndo = true
		v.undoHistory = append(v.undoHistory, newHistoryEntry)
	}
	v.mu.Unlock()

	return nil
}

func (v *VaultService) handleRedoDelete(ctx context.Context, entry historyEntry) error {
	deletedDbEntry, err := v.dbQ.DeleteEntry(ctx, db.DeleteEntryParams{
		Domain:   entry.payload.Domain,
		Username: entry.payload.Username,
	})
	if err != nil {
		return err
	}

	newHistoryEntry := historyEntry{
		kind:    redoActionRegister,
		payload: deletedDbEntry,
	}
	v.mu.Lock()
	if entry.isUndo {
		newHistoryEntry.isUndo = false
		v.redoHistory = append(v.redoHistory, newHistoryEntry)
	} else {
		newHistoryEntry.isUndo = true
		v.undoHistory = append(v.undoHistory, newHistoryEntry)
	}
	v.mu.Unlock()
	return nil
}

func (v *VaultService) handleRedoUpdate(ctx context.Context, entry historyEntry) error {

	oldEntry, err := v.dbQ.GetEntry(ctx, db.GetEntryParams{
		Domain:   entry.payload.Domain,
		Username: entry.payload.Username,
	})
	if err != nil {
		return err
	}

	_, err = v.dbQ.UpdateEntry(ctx, db.UpdateEntryParams{
		Domain:            entry.payload.Domain,
		Username:          entry.payload.Username,
		EncryptedPassword: entry.payload.EncryptedPassword,
		UpdatedAt:         entry.payload.UpdatedAt,
	})
	if err != nil {
		return err
	}

	newHistoryEntry := historyEntry{
		kind:    redoActionUpdate,
		payload: oldEntry,
	}

	v.mu.Lock()
	if entry.isUndo {
		newHistoryEntry.isUndo = false
		v.redoHistory = append(v.redoHistory, newHistoryEntry)
	} else {
		newHistoryEntry.isUndo = true
		v.undoHistory = append(v.undoHistory, newHistoryEntry)
	}
	v.mu.Unlock()
	return nil
}
