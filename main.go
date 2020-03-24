package main

import (
	"archive/zip"
	"fmt"
	"io"
	"io/ioutil"
	"net/http"
	"os"
	"strconv"
	"sync"

	"github.com/PuerkitoBio/goquery"
	"github.com/go-redis/redis"
)

const (
	novel_url = "http://www.janpn.com/book/15/15800/"
)

type NovelItem struct {
	Title string
	Url   string
	Id    string
}

var (
	NovelCh    chan NovelItem
	NovelLen   int
	wg         sync.WaitGroup
	wn         sync.Once
	AkaRds     *redis.Client
	NovelTitle string
	zipOK      = false
)

// connect redis
func bootRds() error {
	wn.Do(func() {
		AkaRds = redis.NewClient(&redis.Options{
			Addr:     "127.0.0.1:6379",
			Password: "",
			DB:       0,
		})
	})
	return AkaRds.Ping().Err()
}

// 爬取小说文章页并发放给通道
func GetNovel(url string) {
	novel_response, left := http.Get(url)
	if left != nil {
		panic(left)
	}
	defer novel_response.Body.Close()
	if novel_response.StatusCode != 200 {
		panic(fmt.Sprintf("http status code is %d", novel_response.StatusCode))
	}
	gqAka, left := goquery.NewDocumentFromReader(novel_response.Body)
	if left != nil {
		panic(left)
	}

	filter := gqAka.Find(".panel-chapterlist li")
	resLen := filter.Length()
	NovelLen = resLen
	if resLen > 0 {
		NovelCh = make(chan NovelItem, resLen)
		NovelTitle = gqAka.Find("title").Text()
		filter.Each(func(i int, s *goquery.Selection) {
			aEle := s.Find("a")
			aHref, _ := aEle.Attr("href")
			if aHref != "" {
				aHref = fmt.Sprintf("%s%s", novel_url, aHref)
			}
			NovelCh <- NovelItem{Title: aEle.Text(), Url: aHref, Id: strconv.Itoa(i)}

		})
	}
	if NovelCh != nil {
		close(NovelCh)
	}

}

// 爬取小说内容页并写入redis
func DoTask(novel NovelItem) {
	defer wg.Done()
	novel_response, left := http.Get(novel.Url)
	if left != nil {
		panic(left)
	}
	defer novel_response.Body.Close()
	if novel_response.StatusCode != 200 {
		fmt.Printf("http status code is %d and url is %s\n", novel_response.StatusCode, novel.Url)
		return
	}
	gqAka, left := goquery.NewDocumentFromReader(novel_response.Body)
	if left != nil {
		panic(left)
	}
	rdsKey := "aka_" + novel.Id
	novelContent := gqAka.Find("#htmlContent").Text()
	AkaRds.Set(rdsKey, novelContent, 0) // 存入redis string
}

// 产生两层goroutine 这是第二层 用于执行爬取文章详情页
func GetNovelTask() {
	select {
	case novelItem := <-NovelCh:
		if novelItem.Url != "" {
			wg.Add(1)
			go DoTask(novelItem)
		}
	default:
	}
	wg.Done()
}

// exec read from redis and create txt file and delete redis key
func fromRdsToFile(writerTxt io.Writer, akaKey string) {
	novelContent := AkaRds.Get(akaKey).Val()
	AkaRds.Del(akaKey)
	defer wg.Done()
	writerTxt.Write([]byte(novelContent))
}

func compressZip(readList []os.FileInfo, txtPath string, zw *zip.Writer) {
	for _, xInfo := range readList {
		if !xInfo.IsDir() {
			frName := txtPath + "/" + xInfo.Name()
			fr, err := os.Open(frName)
			if err != nil {
				fmt.Println(err)
				continue
			}
			fi, err := fr.Stat()
			if err != nil {
				fr.Close()
				fmt.Println(err)
				continue
			}
			fh, err := zip.FileInfoHeader(fi)
			w, err := zw.CreateHeader(fh)
			if err != nil {
				fr.Close()
				fmt.Println(err)
				continue
			}
			_, err = io.Copy(w, fr)
			if err != nil {
				fr.Close()
				fmt.Println(err)
				continue
			}

			fr.Close()
		}
	}
}

func main() {
	if bootRds := bootRds(); bootRds != nil {
		panic(bootRds)
	}
	defer AkaRds.Close()
	GetNovel(novel_url) // 爬取目录页 goquery过滤 写入通道
	fPath, _ := os.Getwd()
	fmt.Println(fPath)
	if NovelLen > 0 {
		fmt.Printf("获得了%d条小说章节 开始爬取\n", NovelLen)
		wg.Add(NovelLen)
		for i := 0; i < NovelLen; i++ {
			go GetNovelTask()
		}
		wg.Wait()

		fmt.Println("爬取完毕...正在写入")

		rdsKeys := AkaRds.Keys("aka_*").Val()
		rdsKeyLen := len(rdsKeys)

		if rdsKeyLen < 1 {
			fmt.Println("DEBUG")
			os.Exit(2)
		}
		wg.Add(rdsKeyLen)
		txtApdFileName := fmt.Sprintf("%s%s", fPath, "/novels/noveltxt/")
		//fmt.Println(txtApdFileName)
		_, err := os.Stat(txtApdFileName)
		if err != nil {
			err := os.Mkdir(txtApdFileName, 0777)
			if err != nil {
				fmt.Println(err)
			}
		}
		txtApdFileNameMix := fmt.Sprintf("%s%s.txt", txtApdFileName, NovelTitle)
		txtApdFile, err := os.OpenFile(txtApdFileNameMix, os.O_WRONLY|os.O_CREATE|os.O_APPEND, 0644)
		if err != nil {
			fmt.Println(err)
		}
		defer txtApdFile.Close()
		for i := 0; i < rdsKeyLen; i++ {
			go fromRdsToFile(txtApdFile, rdsKeys[i])
		}
		wg.Wait()
		fmt.Printf("执行完毕,共记录小说%d章\n", rdsKeyLen)

	} else {
		fmt.Println("未知错误")
		os.Exit(2)
	}

	if zipOK {
		zipPath := fPath + "/novels/aka.zip"
		zipAka, err := os.Create(zipPath)
		if err != nil {
			panic(err)
		}
		defer zipAka.Close()
		zw := zip.NewWriter(zipAka)
		defer func() {
			// 检测一下是否成功关闭
			if err := zw.Close(); err != nil {
				fmt.Println(err)
			}
		}()
		txtPath := fPath + "/novels/noveltxt"
		readList, err := ioutil.ReadDir(txtPath) // readList []FileInfo
		compressZip(readList, txtPath, zw)       // 遍历文件夹下txt文件写入zip header
		fmt.Println("压缩完毕")
	}
}
