package main

import (
	"os"
	"sync"
)

var (
	NovelLen  int
	novel_url string
	NovelCh   chan NovelItem
	wg        sync.WaitGroup
	novelLock sync.Mutex
	fafaLog   *os.File

	NovelTitle string
	zigzagPath string
	logPath    string

	zipOK            = false
	zaddForNovelsKey = "novels"
)

type NovelItem struct {
	Title string
	Url   string
	Id    string
}
