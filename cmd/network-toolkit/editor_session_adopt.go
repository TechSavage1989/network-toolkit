package main

// The main window's editor and the standalone Text Editor each keep a session
// (open tabs and kept copies of unsaved text) in their own workspace folder.
// So that what one of them kept is reachable from the other, an editor that
// restores its session also takes over the other workspace's session when
// that other app is not running (docs/SHARED_APP_DATA.md):
//
//   - every process that uses its editor session holds a shared OS lock on
//     Network_Toolkit.editor-session.lock in its workspace for as long as it
//     runs. Taking over needs that lock exclusively, so a running editor is
//     never taken from. When both run, each keeps its own.
//   - a tab is moved, never copied: its record leaves the other index only
//     after this workspace's index holds it, and a kept copy is removed there
//     only after it exists here. A run that is interrupted at any point is
//     completed by the next one without a duplicate and without losing a
//     kept copy, whichever of the two apps starts next.
import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"
)

const editorSessionLeaseName = "Network_Toolkit.editor-session.lock"

// How long a starting editor waits for another app that is taking tabs over
// from its workspace at that moment.
var editorSessionLeaseWait = 15 * time.Second

var errEditorSessionTakeover = errors.New("Another Network Toolkit window is taking over the open editor files. Try again in a moment.")

// A session folder can only be linked or renamed into within one volume;
// tests replace this to exercise the copying path.
var editorSessionLink = os.Link

// holdEditorSessionLease marks this process as using its workspace's editor
// session until it exits. It waits while another app is taking tabs over
// from this workspace, so the session read afterwards is the settled one.
func (a *application) holdEditorSessionLease() error {
	s := &a.editorSession
	s.leaseMu.Lock()
	defer s.leaseMu.Unlock()
	if s.lease != nil {
		return nil
	}
	workspace, err := openStorageRoot(filepath.Dir(a.dataPath), true)
	if err != nil {
		return err
	}
	defer workspace.Close()
	file, err := openStorageRegular(workspace, editorSessionLeaseName, os.O_CREATE|os.O_RDWR, 0600)
	if err != nil {
		return err
	}
	deadline := time.Now().Add(editorSessionLeaseWait)
	for {
		err = lockStorageFile(file, false)
		if err == nil {
			break
		}
		if !errors.Is(err, ErrAppStorageBusy) || time.Now().After(deadline) {
			_ = file.Close()
			if errors.Is(err, ErrAppStorageBusy) {
				return errEditorSessionTakeover
			}
			return err
		}
		time.Sleep(50 * time.Millisecond)
	}
	s.lease = file
	if a.ctx != nil {
		go func() {
			<-a.ctx.Done()
			s.leaseMu.Lock()
			defer s.leaseMu.Unlock()
			// The file is closed in any case: an open handle keeps the folder
			// from being removed on Windows.
			_ = unlockStorageFile(file)
			_ = file.Close()
			if s.lease == file {
				s.lease = nil
			}
		}()
	}
	return nil
}

// otherEditorWorkspace returns the folder and window mode of the other app
// that keeps an editor session for the same data folder.
func (a *application) otherEditorWorkspace() (dir, mode string) {
	a.ensureShared()
	switch {
	case a.windowMode == standaloneEditorMode && a.shared.standalone:
		return a.shared.root, "main"
	case a.windowMode == "main" && !a.shared.standalone:
		return filepath.Join(filepath.Dir(a.dataPath), standaloneEditorDirectory), standaloneEditorMode
	}
	return "", ""
}

func editorSessionBaseName(path string) string {
	if i := strings.LastIndexAny(path, `/\`); i >= 0 {
		path = path[i+1:]
	}
	if !validEditorSessionLabel(path, 255) {
		return "Untitled.txt"
	}
	return path
}

// adoptEditorSessionBackup brings one kept copy into this session folder
// under id. The copy in the other folder is left in place.
func adoptEditorSessionBackup(from, to *storageRoot, backup editorSessionBackup, id string) error {
	source, target := editorSessionBackupFile(backup.ID), editorSessionBackupFile(id)
	info, err := from.Lstat(source)
	if err != nil {
		return err
	}
	if !info.Mode().IsRegular() || storageIsLink(info) || info.Size() != backup.Size {
		return errors.New("the kept copy is missing or changed")
	}
	if existing, err := to.Lstat(target); err == nil {
		if existing.Mode().IsRegular() && !storageIsLink(existing) && existing.Size() == backup.Size {
			return nil // Brought over by an earlier run that was interrupted.
		}
		return errors.New("a different kept copy already has this name")
	} else if !errors.Is(err, os.ErrNotExist) {
		return err
	}
	sourcePath, err := from.checkedPath(source, true)
	if err != nil {
		return err
	}
	targetPath, err := to.checkedPath(target, true)
	if err != nil {
		return err
	}
	// Both folders are in the data folder, so a second name for the same
	// file is immediate and needs no extra space, whatever the size.
	if editorSessionLink(sourcePath, targetPath) == nil {
		return nil
	}
	in, err := openStorageRegular(from, source, os.O_RDONLY, 0)
	if err != nil {
		return err
	}
	defer in.Close()
	temporary := ".backup-" + randomHex(16) + ".tmp"
	out, err := to.OpenFile(temporary, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0600)
	if err != nil {
		return err
	}
	defer to.Remove(temporary)
	hash := sha256.New()
	written, err := io.Copy(io.MultiWriter(out, hash), in)
	if err == nil && (written != backup.Size || hex.EncodeToString(hash.Sum(nil)) != backup.SHA256) {
		err = errors.New("the kept copy is damaged")
	}
	if err == nil {
		err = out.Sync()
	}
	if err = errors.Join(err, out.Close()); err != nil {
		return err
	}
	return to.Rename(temporary, target)
}

// adoptEditorSessionLocked moves the other editor workspace's tabs and kept
// copies into this workspace's session when that app is not running. It
// returns how many tabs were brought over. Callers hold editorSession.mu.
func (a *application) adoptEditorSessionLocked(ctx context.Context) (int, error) {
	otherDir, otherMode := a.otherEditorWorkspace()
	if otherDir == "" {
		return 0, nil
	}
	other, err := openStorageRoot(otherDir, false)
	if err != nil {
		return 0, nil // That app has never been used.
	}
	defer other.Close()
	otherSession, err := openStorageChild(other, editorSessionDirectory)
	if err != nil {
		return 0, nil
	}
	defer otherSession.Close()
	if _, err := otherSession.Lstat(editorSessionIndexName); err != nil {
		return 0, nil
	}
	// A version of the app from before sharing does not hold the lock below;
	// ask its window directly. Nothing is taken from a running app.
	if activateWorkspaceWindow(ctx, otherDir, otherMode, editorLaunchEndpoint, nil) == nil {
		return 0, nil
	}
	lease, err := openStorageRegular(other, editorSessionLeaseName, os.O_CREATE|os.O_RDWR, 0600)
	if err != nil {
		return 0, nil
	}
	defer lease.Close()
	if err := lockStorageFile(lease, true); err != nil {
		return 0, nil // The other editor is running: each keeps its own session.
	}
	defer unlockStorageFile(lease)
	theirs, err := readEditorSessionIndex(otherSession)
	if err != nil || len(theirs.Tabs) == 0 {
		return 0, nil // An unreadable session is left for its own app to set aside.
	}
	own, err := a.editorSessionRoot(true)
	if err != nil {
		return 0, err
	}
	defer own.Close()
	mine, _, err := a.readEditorSessionLocked(own)
	if err != nil {
		return 0, err
	}
	s := &a.editorSession
	paths, copies := map[string]bool{}, map[string]editorSessionBackup{}
	for _, tab := range mine.Tabs {
		if tab.Kind == "local" {
			paths[tab.Path] = true
		}
		if tab.Backup != nil {
			copies[tab.Backup.ID] = *tab.Backup
		}
	}
	merged := editorSessionIndex{Version: 1, Active: mine.Active, Tabs: append([]editorSessionTab{}, mine.Tabs...)}
	var stay []editorSessionTab // Records that remain in the other session.
	var moved []string          // Kept copies to remove there afterwards.
	var created []string        // Kept copies created here by this run.
	adopted, active := 0, -1
	for index, tab := range theirs.Tabs {
		if len(merged.Tabs) >= editorSessionTabLimit {
			stay = append(stay, tab) // No room now: it stays where it is, untouched.
			continue
		}
		if tab.Backup == nil {
			// A saved file to reopen. If it is open here already there is
			// nothing to bring over.
			if !paths[tab.Path] {
				paths[tab.Path] = true
				merged.Tabs = append(merged.Tabs, tab)
				adopted++
				if index == theirs.Active {
					active = len(merged.Tabs) - 1
				}
			}
			continue
		}
		backup := *tab.Backup
		if held, ok := copies[backup.ID]; ok && held.Size == backup.Size && held.SHA256 == backup.SHA256 {
			// An earlier run brought this tab over and was interrupted before
			// it could take it out of the other session.
			moved = append(moved, backup.ID)
			continue
		}
		id := backup.ID
		if _, taken := copies[id]; taken {
			id = randomHex(16)
		}
		s.pending[id], s.known[id] = time.Now(), true
		_, existed := own.Lstat(editorSessionBackupFile(id))
		if err := adoptEditorSessionBackup(otherSession, own, backup, id); err != nil {
			delete(s.pending, id)
			stay = append(stay, tab) // A copy that cannot be read stays with its record.
			continue
		}
		if existed != nil {
			created = append(created, id)
		}
		record := tab
		record.Backup = &editorSessionBackup{ID: id, Size: backup.Size, SHA256: backup.SHA256}
		if tab.Kind == "local" && paths[tab.Path] {
			// The same file is open here too. The other window's unsaved text
			// comes along as a copy; neither version is dropped.
			origin := tab.Path
			if !validEditorSessionLabel(origin, 1024) {
				origin = editorSessionBaseName(tab.Path)
			}
			record = editorSessionTab{Kind: "copy", Name: editorSessionBaseName(tab.Path), Origin: origin, Language: tab.Language, Dirty: true, Format: tab.Format, Backup: record.Backup}
		} else if tab.Kind == "local" {
			paths[tab.Path] = true
		}
		copies[id] = *record.Backup
		merged.Tabs = append(merged.Tabs, record)
		moved = append(moved, backup.ID)
		adopted++
		if index == theirs.Active {
			active = len(merged.Tabs) - 1
		}
	}
	if len(mine.Tabs) == 0 && active >= 0 {
		merged.Active = active
	}
	if adopted == 0 && len(moved) == 0 && len(stay) == len(theirs.Tabs) {
		return 0, nil // Nothing can be brought over now; the other session is untouched.
	}
	undo := func() {
		for _, id := range created {
			_ = own.Remove(editorSessionBackupFile(id))
			delete(s.pending, id)
		}
	}
	if err := validateEditorSession(merged); err != nil {
		undo()
		return 0, nil
	}
	// 1. This workspace's index now holds the tabs.
	if adopted > 0 {
		if err := a.writeEditorSessionLocked(own, merged); err != nil {
			undo()
			return 0, err
		}
	}
	// 2. They leave the other index. 3. Their copies are removed there.
	remaining := editorSessionIndex{Version: 1, Active: -1, Tabs: stay}
	if remaining.Tabs == nil {
		remaining.Tabs = []editorSessionTab{}
	}
	raw, err := json.MarshalIndent(remaining, "", "  ")
	if err != nil {
		return adopted, nil
	}
	if err := writeStorageAtomicLimit(otherSession, editorSessionIndexName, raw, editorSessionIndexLimit); err != nil {
		// The tabs are in both sessions for now; the next start of either app
		// finds them here and completes the move.
		return adopted, nil
	}
	for _, id := range moved {
		name := editorSessionBackupFile(id)
		if info, err := otherSession.Lstat(name); err == nil && info.Mode().IsRegular() && !storageIsLink(info) {
			_ = otherSession.Remove(name)
		}
	}
	return adopted, nil
}

func editorSessionAdoptionNote(count int, mode string) string {
	if count == 0 {
		return ""
	}
	from := "the main Network Toolkit window"
	if mode == standaloneEditorMode {
		from = "the Text Editor window"
	}
	if count == 1 {
		return "Also reopened 1 file that was open in " + from + "."
	}
	return fmt.Sprintf("Also reopened %d files that were open in %s.", count, from)
}
