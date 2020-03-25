package main

import (
	"os"
	"sync"
)

var (
	NovelLen int
	NovelCh  chan NovelItem
	wg       sync.WaitGroup
	novelLock sync.Mutex
	fafaLog       *os.File

	NovelTitle    string
	NovelSavePath string
	logPath       string

	zipOK            = false
	novel_url        = "http://www.janpn.com/book/238/238381/"
	zaddForNovelsKey = "novels"
)

type NovelItem struct {
	Title string
	Url   string
	Id    string
}
