/*
	gobal - config.go

	Configuration file loading and parsing.

	Copyright (c) 2012-2013 by authors and contributors.
*/

package main

import (
	"bufio"
	"errors"
	"fmt"
	"io"
	"os"
	"regexp"
	"strings"
)

// Interactor is the interface for things that can be set and turned on. This
// is defined for services, pools, etc to implement a common interface for the
// configuration system.
type Interactor interface {
	Set(string, string) error
	Enable() error
}

type ConfigFunc func(*Interactor, []string) error

// To add configuration items, you can add to this map directly if you are a
// user. If you are a plugin, you can add to it via your init function. You can
// pass closures, of course, or a method pointer. Any error returned is fatal
// and we stop processing and shut down.
var ConfigMap map[string]ConfigFunc = map[string]ConfigFunc{
	`^CREATE\s+SERVICE\s+(\w+)$`:       cfg_CreateService,
	`^CREATE\s+POOL\s+(\w+)$`:          cfg_CreatePool,
	`^SET\s+(\w+\.)?(\w+)\s*=\s*(.+)$`: cfg_Set,
	`^ENABLE\s+(\w+)$`:                 cfg_Enable,
	`^DEFAULT\s+(\w+)\s*=\s*(.+)$`:     cfg_Default,
}

// cfg_Default sets a default. These apply to newly created services.
func cfg_Default(cur *Interactor, m []string) error {
	ServiceDefault(m[1], m[2])
	return nil
}

// cfg_Set sets a variable on something. We assume that anything that can be
// set obeys the Interactor interface.
func cfg_Set(cur *Interactor, m []string) error {
	if m[1] == "" {
		// Not specified, use current.
		if cur == nil {
			return errors.New("attempt to set, but no service defined")
		}
		return (*cur).Set(m[2], m[3])
	}

	// Specified, load specific service.
	mcur, ok := services[m[1]]
	if !ok {
		return errors.New(fmt.Sprintf("service '%s' not found", m[1]))
	}
	return mcur.Set(m[2], m[3])
}

// cfg_Enable finishes the configuration of an object and starts it up.
func cfg_Enable(cur *Interactor, m []string) error {
	mcur, ok := services[m[1]]
	if !ok {
		return errors.New(fmt.Sprintf("service '%s' not found", m[1]))
	}
	return mcur.Enable()
}

// cfg_CreateService creates a new service of a given name.
func cfg_CreateService(cur *Interactor, m []string) error {
	svc, err := NewService(m[1])
	if err != nil {
		return err
	}

	*cur = svc
	return nil
}

// cfg_CreatePool creates a new pool of a given name.
func cfg_CreatePool(cur *Interactor, m []string) error {
	svc, err := NewPool(m[1])
	if err != nil {
		return err
	}

	*cur = svc
	return nil
}

func loadConfig(file string) error {
	if file == "" {
		return errors.New("configuration file required")
	}

	f, err := os.Open(file)
	if err != nil {
		return err
	}

	var current Interactor
	eof := false
	rdr := bufio.NewReader(f)
	for {
		if eof {
			break
		}

		line, ferr := rdr.ReadString('\n')
		if ferr != nil && ferr != io.EOF {
			return err
		} else if ferr == io.EOF {
			eof = true
		}
		log.Debug("[CONFIG] %s", line)

		// Remove all whitespace front and rear, and ignore lines that start
		// with a comment sign.
		line = strings.TrimSpace(line)
		if strings.HasPrefix(line, "#") {
			continue
		} else if line == "" {
			continue
		}

		// Now iterate over our config map. This is very slow, but we're talking
		// small numbers of N and is just a startup cost, so it shouldn't matter
		// much at the end of the day.
		any := false
		for str, fnc := range ConfigMap {
			re, err := regexp.Compile("(?i:" + str + ")")
			if err != nil {
				return err
			}

			m := re.FindStringSubmatch(line)
			if m != nil {
				err = fnc(&current, m)
				if err != nil {
					return err
				}
				any = true
				break
			}
		}
		if !any {
			return errors.New(fmt.Sprintf("invalid config: %s", line))
		}
	}
	return nil
}
