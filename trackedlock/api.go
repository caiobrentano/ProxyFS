package trackedlock

import (
	"fmt"
	"sync"

	"github.com/swiftstack/ProxyFS/dlm"
	"github.com/swiftstack/ProxyFS/logger"
)

/*
 * trackedlock provides an implementation of sync.Mutex and sync.RWMutex that
 * track how the locks are used.
 *
 * Specifically, if enabled, the first thing checked is the lock hold time (if
 * enabled).  When a lock unlocked, if it was held longer than
 * "LockHoldTimeLimit" then a warning is logged along with the stack trace of
 * the Lock() and Unlock() of the lock.  In addition, a daemon, the trackedlock
 * watcher, periodically checks to see if any lock has been locked too long.
 * When a lock is held too long, the daemon logs the goroutine ID and the stack
 * trace of the goroutine that acquired the lock.
 *
 * config variable "TrackedLock.LockHoldTimeLimit" is the hold time that
 * triggers warning messages being logged.  If it is 0 then locks are not
 * tracked and the overhead of this package is minimal.
 *
 * config variable "TrackedLock.LockCheckPeriod" is how often the daemon checks
 * the tracked locks.  If it is 0 then no daemon is created and lock hold time
 * is checked only when the lock is unlocked (assuming it is unlocked).
 *
 * trackedlock locks can be locked before the package is initialized, but they
 * will not be tracked until the first time they are locked after
 * initializaiton.
 *
 * The API consists of the config based Up()/Down()/PauseAndContract()/
 * ExpandAndResume() and then the Mutex and RWMutex interface, so there's really
 * nothing to put in this file (unless we simply define the interfaces
 * capitalised and then redirect them to lowercase versions of the same).
 */

// The Mutex type that we export, which wraps sync.Mutex to add tracking of lock
// hold time and the stack trace of the locker.
//
type Mutex struct {
	wrappedMutex sync.Mutex // the actual Mutex
	tracker      mutexTrack // tracking information for the Mutex
}

// The RWMutex type that we export, which wraps sync.RWMutex to add tracking of
// lock hold time and the stack trace of the locker.
//
type RWMutex struct {
	wrappedRWMutex sync.RWMutex // actual Mutex
	rwTracker      rwMutexTrack // track holds in shared (reader) mode
}

// The DLM RWLockStruct that we exports, which wraps dlm.RWLockStruct to add
// tracking lock hold time and the stack trace of the locker.
//
type RWLockStruct struct {
	wrappedRWLock dlm.RWLockStruct // actual DLM lock
	rwTracker     rwMutexTrack     // track holds in shared (reader) mode
}

//
// Tracked Mutex API
//
func (m *Mutex) Lock() {
	m.tracker.Lock(&m.wrappedMutex, m)
}

func (m *Mutex) Unlock() {
	m.tracker.Unlock(&m.wrappedMutex, m)
}

//
// Tracked RWMutex API
//
func (m *RWMutex) Lock() {
	m.rwTracker.Lock(&m.wrappedRWMutex, m)
}

func (m *RWMutex) Unlock() {
	m.rwTracker.Unlock(&m.wrappedRWMutex, m)
}

func (m *RWMutex) RLock() {
	m.rwTracker.RLock(&m.wrappedRWMutex, m)
}

func (m *RWMutex) RUnlock() {
	m.rwTracker.RUnlock(&m.wrappedRWMutex, m)
}

//
// Tracked dlm.RWLockStruct implementation
//
func (m *RWLockStruct) GetLockID() string {
	return m.wrappedRWLock.GetLockID()
}

func (m *RWLockStruct) GetCallerID() dlm.CallerID {
	return m.wrappedRWLock.GetCallerID()
}

func (m *RWLockStruct) IsReadHeld() bool {
	return m.wrappedRWLock.IsReadHeld()
}

func (m *RWLockStruct) IsWriteHeld() bool {
	return m.wrappedRWLock.IsWriteHeld()
}

func (m *RWLockStruct) WriteLock() (err error) {

	// rwLockWrapper maps the RWMutex API to the DLM RWLockStruct API so we
	// can use the RWMutex tracker directly
	rwLockWrapper := DLMLockToRWMutex{&m.wrappedRWLock}
	m.rwTracker.Lock(&rwLockWrapper, m)
	return
}

func (m *RWLockStruct) ReadLock() (err error) {
	rwLockWrapper := DLMLockToRWMutex{&m.wrappedRWLock}
	m.rwTracker.RLock(&rwLockWrapper, m)
	return
}

func (m *RWLockStruct) Unlock() (err error) {
	rwLockWrapper := DLMLockToRWMutex{&m.wrappedRWLock}

	// Use m.tracker.lockCnt without holding the mutex that protects it.
	// Because this goroutine holds the lock, it cannot change from -1 to 0
	// or >0 to 0 (unless there's a bug where another goroutine releases the
	// lock).  It can change from, say, 1 to 2 or 4 to 3, but that's benign
	// (let's hope the race detector doesn't complain).
	switch {
	case m.rwTracker.tracker.lockCnt == -1:
		m.rwTracker.Unlock(&rwLockWrapper, m)
	case m.rwTracker.tracker.lockCnt > 0:
		m.rwTracker.RUnlock(&rwLockWrapper, m)
	default:
		errstring := fmt.Errorf("tracker for RWLockStruct has illegal lockCnt %d",
			m.rwTracker.tracker.lockCnt)
		logger.PanicfWithError(errstring, "RWLockStruct (DLM) lock at %p", &m.wrappedRWLock)
	}
	return
}

func (m *RWLockStruct) TryWriteLock() (err error) {
	err = m.wrappedRWLock.TryWriteLock()
	if err != nil {
		// should really check to see if this EAGAIN and panic if any
		// other error is returned, but leave that to the caller
		return
	}

	// we've already got the lock so give the tracker a dummy lock to lock
	var dummyLock RWMutexNoop
	m.rwTracker.tracker.Lock(&dummyLock, m)
	return
}

func (m *RWLockStruct) TryReadLock() (err error) {
	err = m.wrappedRWLock.TryReadLock()
	if err != nil {
		// should really check to see if this EAGAIN and panic if any
		// other error is returned, but leave that to the caller
		return
	}

	// we've already got the lock so give the tracker a dummy lock to lock
	var dummyLock RWMutexNoop
	m.rwTracker.RLock(&dummyLock, m)
	return
}

// DLMLockToRWMutex corrects the impedance mismatch between DLM locks and
// RWMutex API so DLM locks can be used with same tracker code that we use for
// RWMutex.
//
// Any error returned by the DLM locks is fatal.
//
type DLMLockToRWMutex struct {
	rwLockPtr *dlm.RWLockStruct
}

func (l *DLMLockToRWMutex) Lock() {
	err := l.rwLockPtr.WriteLock()
	if err != nil {
		logger.PanicfWithError(err, "WriteLock() failed on lock at %p", l.rwLockPtr)
	}
	return
}

func (l *DLMLockToRWMutex) Unlock() {
	err := l.rwLockPtr.Unlock()
	if err != nil {
		logger.PanicfWithError(err, "Unlock() failed on lock at %p", l.rwLockPtr)
	}
	return
}

func (l *DLMLockToRWMutex) RLock() {
	err := l.rwLockPtr.ReadLock()
	if err != nil {
		logger.PanicfWithError(err, "ReadLock() failed on lock at %p", l.rwLockPtr)
	}
	return
}

func (l *DLMLockToRWMutex) RUnlock() {
	err := l.rwLockPtr.Unlock()
	if err != nil {
		logger.PanicfWithError(err, "RUnlock() failed on lock at %p", l.rwLockPtr)
	}
	return
}

// RWMutexNoop implements the RWMutex interface as a no-op.
//
// It's useful to implement the the trylock interface of DLM locks where we
// acquire the lock before calling the tracker but want to give the tracker
// something to "lock".
//
type RWMutexNoop struct {
}

func (l *RWMutexNoop) Lock() {
	return
}

func (l *RWMutexNoop) Unlock() {
	return
}

func (l *RWMutexNoop) RLock() {
	return
}

func (l *RWMutexNoop) RUnlock() {
	return
}
