// From https://github.com/bronze1man/kmg
// Package fsnotify implements filesystem notification,.
package fsnotify

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/howeyc/fsnotify"
)

// it is a wrapper of github.com/howeyc/fsnotify.Watcher
// add recursion watch directory.
// add buffer to event chan
// add time to event

type Watcher struct {
	*fsnotify.Watcher
	Event chan *FileEvent
	Error chan error
	//default ignore all file start with "."
	IsIgnorePath func(path string) bool
	//default is nil,if is nil ,error send through Error chan,if is not nil,error handle by this func
	ErrorHandler func(err error)
	isClosed     bool
	quit         chan bool
}

func NewWatcher(bufferSize int) (watcher *Watcher, err error) {
	origin, err := fsnotify.NewWatcher()
	if err != nil {
		return nil, err
	}
	watcher = &Watcher{
		Watcher:      origin,
		Error:        make(chan error, 10),
		Event:        make(chan *FileEvent, bufferSize),
		IsIgnorePath: DefaultIsIgnorePath,
	}
	go watcher.eventHandle()
	return
}
func (w *Watcher) Close() error {
	if w.isClosed {
		return nil
	}
	err := w.Watcher.Close()
	w.quit <- true
	w.isClosed = true
	return err
}
func (w *Watcher) eventHandle() {
	for {
		select {
		case originEvent := <-w.Watcher.Event:
			fmt.Printf("originEvent %+v\n", originEvent)
			if w.IsIgnorePath(originEvent.Name) {
				continue
			}
			w.Event <- newFileEvent(originEvent)
			//github.com/howeyc/fsnotify has not handle this stuff...
			// you can not stat a delete file...
			if originEvent.IsDelete() {
				continue
			}
			fi, err := os.Stat(originEvent.Name)
			if err != nil {
				//rename send two events,one old file,one new file,here ignore old one
				if os.IsNotExist(err) {
					continue
				}
				w.errorHandle(err)
				continue
			}
			if fi.IsDir() {
				w.WatchRecursive(originEvent.Name)
			}
		case err := <-w.Watcher.Error:
			w.errorHandle(err)
		case _ = <-w.quit:
			break
		}
	}
}
func (w *Watcher) errorHandle(err error) {
	if w.ErrorHandler == nil {
		w.Error <- err
		return
	}
	w.ErrorHandler(err)
}
func (w *Watcher) GetErrorChan() chan error {
	return w.Error
}
func (w *Watcher) GetEventChan() chan *FileEvent {
	return w.Event
}
func (w *Watcher) WatchRecursive(path string) error {
	path, err := filepath.Abs(path)
	if err != nil {
		return err
	}
	folders, err := w.getSubFolders(path)
	for _, v := range folders {
		err = w.Watcher.Watch(v)
		if err != nil {
			return err
		}
	}
	return nil
}

func (w *Watcher) getSubFolders(path string) (paths []string, err error) {
	err = filepath.Walk(path, func(newPath string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		if !info.IsDir() {
			return nil
		}
		if w.IsIgnorePath(newPath) {
			return filepath.SkipDir
		}
		paths = append(paths, newPath)
		return nil
	})
	return paths, err
}
func DefaultIsIgnorePath(path string) bool {
	return IsDotFile(path)
}

func IsDotFile(path string) bool {
	if path == "./" {
		return false
	}
	base := filepath.Base(path)
	if strings.HasPrefix(base, ".") {
		return true
	}
	return false
}
