package dlm

// Configuration variables for DLM

import (
	"github.com/swiftstack/ProxyFS/conf"
	"github.com/swiftstack/ProxyFS/trackedlock"
)

type globalsStruct struct {
	trackedlock.Mutex

	// Map used to store locks owned locally
	// NOTE: This map is protected by the Mutex
	localLockMap map[string]*localLockTrack

	// TODO - channels for STOP and from DLM lock master?
	// is the channel lock one per lock or a global one from DLM?
	// how could it be... probably just one receive thread the locks
	// map, checks bit and releases lock if no one using, otherwise
	// blocks until it is free...
}

var globals globalsStruct

func Up(confMap conf.ConfMap) (err error) {
	// Create map used to store locks
	globals.localLockMap = make(map[string]*localLockTrack)
	return
}

func PauseAndContract(confMap conf.ConfMap) (err error) {
	// Nothing to do here
	err = nil
	return
}

func ExpandAndResume(confMap conf.ConfMap) (err error) {
	// Nothing to do here
	err = nil
	return
}

func Down() (err error) {
	return
}
