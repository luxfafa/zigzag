package main

import (
	"fmt"
	"net/http"
	"os"
	"strconv"
	"sync"

	"github.com/PuerkitoBio/goquery"
	"github.com/go-redis/redis"
)

type NovelItem struct {
	Title string
	Url   string
	Id string
}

var (
	NovelCh  chan NovelItem
	NovelLen int
	wg       sync.WaitGroup
	wn       sync.Once
	AkaRds   *redis.Client
)

const (
	novel_url = "http://www.janpn.com/book/145/145837/"
)

// 爬取小说文章页并发放给通道
func GetNovel(url string) {
	novel_response, left := http.Get(url)
	if left != nil {
		panic(left)
	}
	defer novel_response.Body.Close()
	if novel_response.StatusCode != 200 {
		panic(fmt.Sprintf("http status code is %s", novel_response.StatusCode))
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
		filter.Each(func(i int, s *goquery.Selection) {
			aEle := s.Find("a")
			aHref, _ := aEle.Attr("href")
			if aHref != "" {
				aHref = fmt.Sprintf("%s%s", novel_url, aHref)
			}
			NovelCh <- NovelItem{Title:aEle.Text(), Url:aHref,Id:strconv.Itoa(i)}

		})
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
		fmt.Printf("http status code is %s and url is %s\n", novel_response.StatusCode, novel.Url)
		return
	}
	gqAka, left := goquery.NewDocumentFromReader(novel_response.Body)
	if left != nil {
		panic(left)
	}
	rdsKey := "aka_"+novel.Id
	novelContent := gqAka.Find("#htmlContent").Text()
	AkaRds.Set(rdsKey,novelContent,0)  // 存入redis string
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


// connect redis
func bootRds() error {
	wn.Do(func(){
		AkaRds = redis.NewClient(&redis.Options{
			Addr:     "127.0.0.1:6379",
			Password: "",
			DB:       0,
		})
	})
	return AkaRds.Ping().Err()
}

// exec read from redis and create txt file and delete redis key
func fromRdsToFile(fPath,akaKey string) {
	fPathTxt := fmt.Sprintf("%s%s.txt",fPath,akaKey)
	novelContent:=AkaRds.Get(akaKey).Val()
	AkaRds.Del(akaKey)
	f,_ :=os.Create(fPathTxt)
	defer f.Close()
	defer wg.Done()
	f.WriteString(novelContent)
}

func main() {
	if bootRds := bootRds(); bootRds != nil {
		panic(bootRds)
	}
	defer AkaRds.Close()
	GetNovel(novel_url) // 爬取目录页 goquery过滤 写入通道
	defer close(NovelCh) // 存储目录结构体通道
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
		wg.Add(rdsKeyLen)
		fPath,_ := os.Getwd()
		fPath = fmt.Sprintf("%s%s",fPath,"/snake/novels/")
		for i := 0; i < rdsKeyLen; i++ {
			go fromRdsToFile(fPath,rdsKeys[i])
		}
		wg.Wait()
		fmt.Printf("执行完毕,共写入文件%d个",rdsKeyLen)

	} else {
		fmt.Println("未知错误")
	}

}
