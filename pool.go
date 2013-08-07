/*
	gobal - pool.go

	Defines a Pool object, which is a set of backends that a proxy role Service
	can connect to..

	Copyright (c) 2012-201 by authors and contributors.
*/

package main

import (
	"bufio"
	"errors"
	"fmt"
	"io"
	"os"
	"path"
	"strings"
	"sync"
	"time"
)

// Backend represents a server that we connect to.
type Backend struct {
	Ipport string

	// Internal state management variables
	connectMutex sync.Mutex
	connecting   *HttpBackendConnection
	outstanding  int
	generation   int
}

// Pool manages a collection of Backends. It is responsible for spawning new
// connections as needed.
type Pool struct {
	Name string

	// Internal state management variables
	backends     []*Backend
	backendQueue chan *HttpBackendConnection
	nodeFile     string
	nodeFileLock sync.Mutex
	generation   int
}

var poolLock sync.Mutex
var pools map[string]*Pool = make(map[string]*Pool)

//////////////////////////////////////////////////////////////////////////////
// Backend base implementation
//////////////////////////////////////////////////////////////////////////////

// Connect initiates a connection to this backend if one is not already in
// progress. This call returns immediately; the connect happens in a goroutine.
func (self *Backend) Connect() {
	self.connectMutex.Lock()
	defer self.connectMutex.Unlock()
	if self.connecting != nil {
		return
	}

	// If we're here, we want to actually do the connection now.
	go func() {
		self.connectMutex.Lock()
		//self.connecting = MakeHttpBackend(self)
		self.connectMutex.Unlock()

	}()
}

//////////////////////////////////////////////////////////////////////////////
// Pool base implementation
//////////////////////////////////////////////////////////////////////////////

// NewPool creates a pool with a given name. It is an error to create two
// pools with the same name.
func NewPool(name string) (*Pool, error) {
	poolLock.Lock()
	defer poolLock.Unlock()

	if _, ok := pools[name]; ok {
		return nil, errors.New(fmt.Sprintf("pool '%s' already exists", name))
	}

	p := &Pool{
		Name:         name,
		backendQueue: make(chan *HttpBackendConnection, 1000),
	}
	pools[name] = p

	// This is the nodefile worker. It runs every 10 seconds and watches for
	// changes to our nodefile, reloading as necessary.
	go p.updateNodeFileWorker()

	// The spawner is a look-ahead backend connector which tries to stay ahead
	// of estimated traffic by connecting backends ahead of time.
	//go p.spawner()

	return p, nil
}

// updateNodeFileWorker keeps an eye on the node file this pool uses and, when
// it changes on disk, reloads it. This manages our backends structure.
func (p *Pool) updateNodeFileWorker() {
	p.nodeFileLock.Lock()
	defer p.nodeFileLock.Unlock()

	mtime := time.Unix(0, 0)
	for {
		fi, err := os.Stat(p.nodeFile)
		if err != nil {
			log.Error("failed to stat nodefile: %s", err)
			continue
		}

		newmtime := fi.ModTime()
		if !newmtime.After(mtime) {
			continue
		}

		mtime = newmtime
		newgen := p.generation + 1
		log.Debug("nodefile changed: %s", p.nodeFile)

		// Load in the nodefile and update our backend list
		// TODO: Implement :-)
		fobj, err := os.Open(p.nodeFile)
		if err != nil {
			log.Error("failed to open nodefile: %s", err)
			continue
		}

		eof := false
		buf := bufio.NewReader(fobj)
	LINE:
		for {
			if eof {
				break
			}

			line, err := buf.ReadString('\n')
			if err != nil && err != io.EOF {
				log.Error("failed to read from nodefile: %s", err)
				break
			} else if err == io.EOF {
				eof = true
			}

			line = strings.TrimSpace(line)
			idx := strings.Index(line, "#")
			if idx > -1 {
				line = line[0:idx]
			}
			if len(line) < 7 { // At least "1.2.3.4"!
				continue
			}

			// Refresh an existing structure so we can just keep it and move on.
			for _, bstruct := range p.backends {
				if bstruct.Ipport == line {
					bstruct.generation = newgen
					continue LINE
				}
			}

			// Create a new structure and stick it in our list.
			bend := &Backend{
				Ipport:     line,
				generation: newgen,
			}
			p.backends = append(p.backends, bend)
		}

		time.Sleep(10 * time.Second)
	}

}

// updateNodeFile reads in the node file and updates it. It also sets up a
// goroutine that updates the node file every so often.
func (p *Pool) updateNodeFile(nodefile string) error {
	nodefile = path.Clean(strings.TrimSpace(nodefile))
	fi, err := os.Stat(nodefile)
	if err != nil {
		return err
	}
	if fi.IsDir() {
		return errors.New(fmt.Sprintf("nodefile: %s is a directory", nodefile))
	}

	// Now we have to fetch a lock to update the nodefile, so we don't conflict
	// with the ongoing worker.
	p.nodeFileLock.Lock()
	p.nodeFile = nodefile
	p.nodeFileLock.Unlock()

	return nil
}

// GetBackend returns a handle to a backend. Ideally we return one that is ready
// to go, but if there are none in the queue, we'll start up a new one.
func (p *Pool) GetBackend() *HttpBackendConnection {
	return nil
}

// Set something on a pool.
func (p *Pool) Set(key, value string) error {
	switch key {
	case "nodefile":
		return p.updateNodeFile(value)
	default:
		log.Error("unknown SET %s.%s = %s", p.Name, key, value)
	}
	return nil
}

// Enable turns the pool on. This does nothing, however, since we are always
// enabled and ready to return connections.
func (p *Pool) Enable() error {
	return nil
}
