package trackedlock

/*
 * Test tracked locks.
 *
 * Most of this file is copied from statslogger/config_test.go because its a
 * good place to start.
 */

import (
	"fmt"
	"testing"
	"time"

	"github.com/swiftstack/ProxyFS/conf"
	"github.com/swiftstack/ProxyFS/logger"
)

func TestAPI(t *testing.T) {
	confStrings := []string{
		"TrackedLock.LockHoldTimeLimit=2s",
		"TrackedLock.LockCheckPeriod=1s",

		"Stats.IPAddr=localhost",
		"Stats.UDPPort=52184",
		"Stats.BufferLength=100",
		"Stats.MaxLatency=1s",
	}

	confMap, err := conf.MakeConfMapFromStrings(confStrings)
	if err != nil {
		t.Fatalf("%v", err)
	}

	err = logger.Up(confMap)
	if nil != err {
		tErr := fmt.Sprintf("logger.Up(confMap) failed: %v", err)
		t.Fatalf(tErr)
	}

	// first test -- start with tracking disabled, then bring up enabled
	err = Up(confMap)
	if nil != err {
		tErr := fmt.Sprintf("Up('TrackedLock.LockHoldTimeLimit=1s) failed: %v", err)
		t.Fatalf(tErr)
	}
	if globals.lockHoldTimeLimit != 2*time.Second {
		t.Fatalf("after Up('TrackedLock.LockHoldTimeLimit=1s) globals.lockHoldTimeLimi != 2 sec")
	}

	// err = Down()
	// if nil != err {
	// 	tErr := fmt.Sprintf("Down() 'Trackedlock.Period=0s' failed: %v", err)
	// 	t.Fatalf(tErr)
	// }

	// err = confMap.UpdateFromString("Trackedlock.Period=1s")
	// if nil != err {
	// 	tErr := fmt.Sprintf("UpdateFromString('Trackedlock.Period=1s') failed: %v", err)
	// 	t.Fatalf(tErr)
	// }
	// err = Up(confMap)
	// if nil != err {
	// 	tErr := fmt.Sprintf("trackedlock.Up(Trackedlock.Period=1s) failed: %v", err)
	// 	t.Fatalf(tErr)
	// }
	// if globals.statsLogPeriod != 1*time.Second {
	// 	t.Fatalf("after Up('Trackedlock.Period=1s') globals.statsLogPeriod != 1 sec")
	// }

	// Run the tests
	//
	// "Real" unit tests would verify the information written into the log
	//
	// t.Run("testRetry", testRetry)
	// t.Run("testOps", testOps)
	// testReload(t, confMap)
	testMutex(t, confMap)

	// Shutdown packages

	err = Down()
	if nil != err {
		tErr := fmt.Sprintf("Down() failed: %v", err)
		t.Fatalf(tErr)
	}

	err = logger.Down()
	if nil != err {
		tErr := fmt.Sprintf("stats.Down() failed: %v", err)
		t.Fatalf(tErr)
	}
}

// Test trackedlock for Mutex
//
func testMutex(t *testing.T, confMap conf.ConfMap) {
	var (
		testMutex Mutex
	)

	// verify we can lock and unlock
	testMutex.Lock()
	testMutex.Unlock()

	// verify hold limit detected
	sleepTime, err := time.ParseDuration("3s")
	if err != nil {
		tErr := fmt.Sprintf("ParseDuration('3s') failed: %v", err)
		t.Fatalf(tErr)
	}
	testMutex.Lock()
	time.Sleep(sleepTime)
	testMutex.Unlock()
}

// Make sure we can shutdown and re-enable trackedlock
//
func testReload(t *testing.T, confMap conf.ConfMap) {

	var err error

	// Reload trackedlock with logging disabled
	err = confMap.UpdateFromString("Trackedlock.Period=0s")
	if nil != err {
		tErr := fmt.Sprintf("UpdateFromString('Trackedlock.Period=0s') failed: %v", err)
		t.Fatalf(tErr)
	}

	err = PauseAndContract(confMap)
	if nil != err {
		tErr := fmt.Sprintf("PauseAndContract('Trackedlock.Period=0s') failed: %v", err)
		t.Fatalf(tErr)
	}
	err = ExpandAndResume(confMap)
	if nil != err {
		tErr := fmt.Sprintf("PauseAndContract('Trackedlock.Period=0s') failed: %v", err)
		t.Fatalf(tErr)
	}

	// Enable logging again
	err = confMap.UpdateFromString("Trackedlock.Period=1s")
	if nil != err {
		tErr := fmt.Sprintf("UpdateFromString('Trackedlock.Period=1s') failed: %v", err)
		t.Fatalf(tErr)
	}

	err = PauseAndContract(confMap)
	if nil != err {
		tErr := fmt.Sprintf("PauseAndContract('Trackedlock.Period=1s') failed: %v", err)
		t.Fatalf(tErr)
	}
	err = ExpandAndResume(confMap)
	if nil != err {
		tErr := fmt.Sprintf("PauseAndContract('Trackedlock.Period=1s') failed: %v", err)
		t.Fatalf(tErr)
	}
}
