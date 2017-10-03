package trackedlock

import (
	"fmt"
	"runtime"
	"sync"
	"time"

	"github.com/swiftstack/ProxyFS/logger"
	"github.com/swiftstack/ProxyFS/utils"
)

type globalsStruct struct {
	mapMutex          sync.Mutex                    // protects mutexMap and rwMutexMap
	mutexMap          map[*mutexTrack]interface{}   // the Mutex like locks  being watched
	rwMutexMap        map[*rwMutexTrack]interface{} // the RWMutex like locks being watched
	lockHoldTimeLimit time.Duration                 // locks held longer then this get logged
	lockCheckPeriod   time.Duration                 // check locks once each period
	lockCheckChan     <-chan time.Time              // wait here to check on locks
	stopChan          chan struct{}                 // time to shutdown and go home
	doneChan          chan struct{}                 // shutdown complete
	lockCheckTicker   *time.Ticker                  // ticker for lock check time
}

var globals globalsStruct

// stackTrace holds the stack trace of one thread.  stackTraceBuf is the storage
// required to hold one stack trace.
//
type stackTrace []byte
type stackTraceBuf [4040]byte

// Track a Mutex or RWMutex held in exclusive mode (lockCnt is also used to
// track the number of shared lockers)
//
type mutexTrack struct {
	isWatched           bool          // true if lock is on list of checked mutexes
	lockCnt             int           // 0 if unlocked, -1 locked exclusive, > 0 locked shared
	lockTime            time.Time     // time last lock operation completed
	lockerGoId          uint64        // goroutine ID of the last locker
	lockerStackTrace    stackTrace    // stack trace of curent or last locker
	lockerStackTraceBuf stackTraceBuf // reusable storage for stack trace
}

// Track an RWMutex
//
type rwMutexTrack struct {
	tracker           *mutexTrack           // tracking info when the lock is held exclusive
	sharedStateLock   sync.Mutex            // lock the following fields (shared mode state)
	rLockTime         map[uint64]time.Time  // GoId -> lock acquired time
	rLockerStackTrace map[uint64]stackTrace // GoId -> locker stack trace
}

// RWMutex supports this interface, so we need to wrap DLM locks with the same
// interface to reuse the same tracking code.
//
type rwLocker interface {
	Lock()
	Unlock()
	RLock()
	RUnlock()
}

// Locking a Mutex or locking an RWMutex in exclusive (writer) mode.
//
// Note that holding an RWMutex in exclusive mode insures that no goroutine
// holds it in shared mode.
//
func (mt *mutexTrack) Lock(wrappedMutex sync.Locker, wrapPtr interface{}) {

	// if lock tracking is disabled, just get the lock and record the
	// current time (time.Now() is kind've expensive, but may still be worth
	// getting)
	if globals.lockHoldTimeLimit == 0 {
		wrappedMutex.Lock()

		mt.lockTime = time.Now()
		mt.lockCnt = -1
		return
	}

	// mt.lockCnt is just a hint, but if the lock is locked then let's
	// collect our stack trace now, even though this requires memory
	// allocation
	if mt.lockCnt != 0 {
		stackTrace := make([]byte, 4040, 4040)
		cnt := runtime.Stack(stackTrace, false)

		wrappedMutex.Lock()
		mt.lockerStackTrace = stackTrace[0:cnt]
	} else {

		wrappedMutex.Lock()
		mt.lockerStackTrace = mt.lockerStackTraceBuf[:]
		cnt := runtime.Stack(mt.lockerStackTrace, false)
		mt.lockerStackTrace = mt.lockerStackTrace[0:cnt]
	}
	mt.lockerGoId = utils.StackTraceToGoId(mt.lockerStackTrace)
	mt.lockTime = time.Now()
	mt.lockCnt = -1

	// add to the list of watched mutexes if anybody is watching -- its only
	// at this point that we need to know if this a Mutex or RWMutex, and it
	// only happens once
	if !mt.isWatched && globals.lockCheckPeriod != 0 && globals.lockHoldTimeLimit != 0 {
		globals.mapMutex.Lock()

		switch lockPtr := wrapPtr.(type) {
		case *Mutex:
			globals.mutexMap[mt] = wrapPtr
		case *RWMutex:
			globals.rwMutexMap[&lockPtr.rwTracker] = wrapPtr
		case *RWLockStruct:
			globals.rwMutexMap[&lockPtr.rwTracker] = wrapPtr
		default:
			errstring := fmt.Errorf("lock has an unknown type")
			logger.PanicfWithError(errstring, "mutex type lock at %p", wrapPtr)
		}
		globals.mapMutex.Unlock()
		mt.isWatched = true
	}

	return
}

// Unlocking a Mutex or unlocking an RWMutex held in exclusive (writer) mode
//
func (mt *mutexTrack) Unlock(wrappedMutex sync.Locker, wrapPtr interface{}) {

	if globals.lockHoldTimeLimit == 0 {
		mt.lockCnt = 0
		wrappedMutex.Unlock()
		return
	}

	now := time.Now()
	if now.Sub(mt.lockTime) >= globals.lockHoldTimeLimit {

		var mutexType string
		switch wrapPtr.(type) {
		case *Mutex:
			mutexType = "Mutex"
		case *RWMutex:
			mutexType = "RWMutex"
		case *RWLockStruct:
			mutexType = "RWLockStruct"
		default:
			errstring := fmt.Errorf("lock has an unknown type: '%T'", wrapPtr)
			logger.PanicfWithError(errstring, "mutex type lock at %p", wrapPtr)
		}

		var buf stackTraceBuf
		stackTrace := buf[:]
		cnt := runtime.Stack(stackTrace, false)
		stackTraceString := string(stackTrace[0:cnt])

		logger.Warnf("Unlock(): %s at %p locked for %d sec; stack at Lock() call: %s  stack at Unlock(): %s",
			mutexType, wrapPtr,
			now.Sub(mt.lockTime)/time.Second, string(mt.lockerStackTrace), stackTraceString)
	}

	// release the lock
	mt.lockCnt = 0
	wrappedMutex.Unlock()
	return
}

// Tracking a RWMutex locked exclusive is just like a regular Mutex
//
func (rwmt *rwMutexTrack) Lock(wrappedMutex sync.Locker, wrapPtr interface{}) {
	rwmt.tracker.Lock(wrappedMutex, wrapPtr)
}

func (rwmt *rwMutexTrack) Unlock(wrappedMutex sync.Locker, wrapPtr interface{}) {
	rwmt.tracker.Unlock(wrappedMutex, wrapPtr)
}

// Tracking a RWMutex locked shared is more work
//
func (rwmt *rwMutexTrack) RLock(wrappedRWMutex rwLocker, wrapPtr interface{}) {

	// if lock tracking is disabled, just get the lock and record the
	// current time (time.Now() is kind've expensive, still probably
	// worthwhile.
	//
	// Note that when lock tracking is disabled, mt.lockTime is being
	// updated when the locks is acquired shared and mt.lockCnt is not being
	// updated.
	if globals.lockHoldTimeLimit == 0 {
		wrappedRWMutex.RLock()

		rwmt.tracker.lockTime = time.Now()
		return
	}

	// get the stack trace and goId before getting the shared lock
	// to cut down on lock contention
	var buf stackTraceBuf
	stackTrace := buf[:]
	cnt := runtime.Stack(stackTrace, false)
	stackTrace = stackTrace[0:cnt]
	goId := utils.StackTraceToGoId(stackTrace)

	wrappedRWMutex.RLock()

	// The lock is held as a reader (shared mode) so no goroutine can have
	// it locked exclusive.  Holding rwmt.sharedStateLock is sufficient to
	// insure that no other goroutine is changing rwmt.tracker.lockCnt.
	rwmt.sharedStateLock.Lock()

	rwmt.rLockerStackTrace[goId] = stackTrace
	rwmt.rLockTime[goId] = time.Now()
	rwmt.tracker.lockCnt += 1

	// add to the list of watched mutexes if anybody is watching
	if !rwmt.tracker.isWatched && globals.lockCheckPeriod != 0 && globals.lockHoldTimeLimit != 0 {

		globals.mapMutex.Lock()
		globals.rwMutexMap[rwmt] = wrapPtr
		globals.mapMutex.Unlock()
		rwmt.tracker.isWatched = true
	}
	rwmt.sharedStateLock.Unlock()

	return
}

func (rwmt *rwMutexTrack) RUnlock(wrappedRWMutex rwLocker, wrapPtr interface{}) {

	if globals.lockHoldTimeLimit == 0 {
		rwmt.tracker.lockCnt -= 1
		wrappedRWMutex.RUnlock()
		return
	}

	now := time.Now()
	goId := utils.GetGoId()

	rwmt.sharedStateLock.Lock()

	if now.Sub(rwmt.rLockTime[goId]) >= globals.lockHoldTimeLimit {

		var buf stackTraceBuf
		stackTrace := buf[:]
		cnt := runtime.Stack(stackTrace, false)
		stackTraceString := string(stackTrace[0:cnt])

		logger.Warnf("RUnlock(): RWMutex at %p locked for %d sec; stack at Lock() call: %s  stack at RUnlock(): %s",
			wrapPtr, now.Sub(rwmt.rLockTime[goId])/time.Second,
			rwmt.rLockerStackTrace[goId], stackTraceString)
	}

	delete(rwmt.rLockerStackTrace, goId)
	delete(rwmt.rLockTime, goId)
	rwmt.tracker.lockCnt -= 1
	rwmt.sharedStateLock.Unlock()

	// release the lock
	wrappedRWMutex.RUnlock()
	return
}

func lockWatcher() {
	for {
		select {
		case <-globals.stopChan:
			logger.Infof("trackedlock lock watcher shutting down")
			globals.doneChan <- struct{}{}

		case <-globals.lockCheckChan:
			// fall through and perform checks
		}

		var (
			longestMT       *mutexTrack
			longestRWMT     *rwMutexTrack
			longestDuration time.Duration
			longestGoId     uint64
		)
		now := time.Now()

		// see if any Mutex has been held longer then the limit and, if
		// so, find the one held the longest.
		//
		// looking at mutex.tracker.lockTime is safe even if we don't hold the
		// mutex (in practice, looking at mutex.tracker.lockCnt should be safe
		// as well, though go doesn't guarantee that).
		globals.mapMutex.Lock()
		for mt, _ := range globals.mutexMap {
			if mt.lockCnt == 0 {
				continue
			}
			lockedDuration := now.Sub(mt.lockTime)
			if lockedDuration >= globals.lockHoldTimeLimit && lockedDuration > longestDuration {
				longestMT = mt
				longestDuration = lockedDuration
			}
		}
		globals.mapMutex.Unlock()

		// give other routines a chance to get the lock
		time.Sleep(10 * time.Millisecond)

		// see if any RWMutex has been held longer then the limit and, if
		// so, find the one held the longest.
		//
		// looking at the rwMutex.tracker.* fields is safe per the
		// argument for Mutex, but looking at rTracker maps requires we
		// get rtracker.sharedStateLock for each lock.
		globals.mapMutex.Lock()
		for rwmt, _ := range globals.rwMutexMap {
			if rwmt.tracker.lockCnt == 0 {
				continue
			}

			if rwmt.tracker.lockCnt < 0 {
				lockedDuration := now.Sub(rwmt.tracker.lockTime)
				if lockedDuration >= globals.lockHoldTimeLimit && lockedDuration > longestDuration {
					longestMT = nil
					longestRWMT = rwmt
					longestDuration = lockedDuration
				}
				continue
			}

			rwmt.sharedStateLock.Lock()

			for goId, lockTime := range rwmt.rLockTime {
				lockedDuration := now.Sub(lockTime)
				if lockedDuration >= globals.lockHoldTimeLimit && lockedDuration > longestDuration {
					longestMT = nil
					longestRWMT = rwmt
					longestDuration = lockedDuration
					longestGoId = goId
				}
			}
			rwmt.sharedStateLock.Unlock()
		}
		globals.mapMutex.Unlock()

		if longestMT == nil && longestRWMT == nil {
			continue
		}

		// figure out the lock type, call type, stack trace, etc.
		var (
			mutexType  string
			lockCall   string
			stackTrace string
			lockPtr    interface{}
		)

		// looking at tracker.lockerStackTrace is not strictly
		// safe, so let's hope for the best
		//
		// this should really copy the values and then verify
		// the lock is still locked before printing
		switch {
		case longestMT != nil:
			mutexType = "Mutex"
			lockCall = "Lock()"
			stackTrace = string(longestMT.lockerStackTrace)
			lockPtr = globals.mutexMap[longestMT]
		case longestGoId == 0:
			mutexType = "RWMutex"
			lockCall = "Lock()"
			stackTrace = string(longestRWMT.tracker.lockerStackTrace)
			lockPtr = globals.rwMutexMap[longestRWMT]
		default:
			mutexType = "RWMutex"
			lockCall = "RLock()"
			stackTrace = string(longestRWMT.rLockerStackTrace[longestGoId])
			lockPtr = globals.rwMutexMap[longestRWMT]
		}
		logger.Warnf("trackedlock watcher: %s at %p locked for %d sec; stack at %s call: %s",
			mutexType, lockPtr, uint64(longestDuration/time.Second), lockCall, stackTrace)
	}
}
