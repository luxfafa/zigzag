package main

import (
	"fmt"
	"github.com/PuerkitoBio/goquery"
	"github.com/go-redis/redis"
	"github.com/puthx/zigzag/boot"
	"github.com/puthx/zigzag/tools"
	akaLog "github.com/sirupsen/logrus"
	"io"
	"os"
	"runtime"
	"strconv"
)

func init() {
	defer func() {
		if commandErr := recover();commandErr!=nil {
			switch commandErr.(type) {
			case runtime.Error:
				fmt.Printf("Args Error:%v\n",commandErr)
			default:
				fmt.Printf("Other Error:%v\n",commandErr)
			}
			os.Exit(2)
		}
	}()
	novel_url = os.Args[1]

	NovelPath, left := os.Getwd()  // 创建一些目录与文件
	if left != nil {
		panic(left)
	}
	zigzagPath = NovelPath + "/observation"
	logPath = zigzagPath + "/zigzag.log"
	_,left = os.Stat(zigzagPath+"/")
	if left != nil {  // 文件夹不存在就去创建
		left = os.Mkdir(zigzagPath+"/",0777)
		if left != nil {
			panic(left)
		}
	}
	fafaLog, left = os.OpenFile(logPath, os.O_RDWR|os.O_CREATE|os.O_APPEND, 0644)  // main()中延迟关闭
	if left != nil {
		panic(left)
	}

	akaLog.SetFormatter(&akaLog.JSONFormatter{})  // logrus相关配置
	akaLog.SetOutput(fafaLog)
}

// 爬取小说文章页并发放给通道
func GetNovel(url string) {
	novel_response, err := tools.GetNovelData(url)
	if err != nil {
		akaLog.WithFields(akaLog.Fields{
			"err": err,
			"url": url,
		}).Fatal("爬取文章详情页错误")
		os.Exit(2)
	}
	gqAka, left := goquery.NewDocumentFromReader(novel_response.Body)
	defer novel_response.Body.Close()
	if left != nil {
		akaLog.WithFields(akaLog.Fields{
			"err": left,
			"url": url,
		}).Fatal("GoQuery初始化文档错误")
		os.Exit(2)
	}

	filter := gqAka.Find(".panel-chapterlist li")
	NovelLen = filter.Length()
	if NovelLen > 0 {
		NovelCh = make(chan NovelItem, NovelLen)
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
func DoTask(novel NovelItem, NovelChapterKey string) {
	defer wg.Done()
	novelForSortScore := tools.UrlGetChapter(novel.Url)
	if novelForSortScore < 1 {
		akaLog.WithFields(akaLog.Fields{
			"url": novel.Url,
		}).Warn("分割url作为score失败")
		return
	}
	novel_response, err := tools.GetNovelData(novel.Url)
	if err != nil {
		akaLog.WithFields(akaLog.Fields{
			"err": err,
			"url": novel.Url,
		}).Warn("爬取文章详情页错误")
		return
	}
	gqAka, left := goquery.NewDocumentFromReader(novel_response.Body)
	if left != nil {
		akaLog.WithFields(akaLog.Fields{
			"err": err,
			"url": novel_url,
		}).Warn("GoQuery初始化文档错误")
		return
	}
	//novelsMemberKey := boot.GetNovelsChapterKey(novel.Id)
	novelContent := gqAka.Find("#htmlContent").Text()

	//boot.AkaRds.Set(novelsChapterKey, novelContent, 0) // 存入redis string
	boot.AkaRds.ZAdd(NovelChapterKey, redis.Z{Score: novelForSortScore, Member: novelContent})
	//boot.AkaRds.Sort
}

// 产生两层goroutine 这是第二层 用于执行爬取文章详情页
func GetNovelTask() {
	select {
	case novelItem := <-NovelCh:
		if novelItem.Url != "" {
			wg.Add(1)
			go DoTask(novelItem, zaddForNovelsKey)
		}
	default:
	}
	wg.Done()
}

// exec read from redis and create txt file and delete redis key
func fromRdsToFile(writerTxt io.Writer, novelContent string) {
	defer wg.Done()
	novelLock.Lock()
	writerTxt.Write([]byte(novelContent))
	novelLock.Unlock()
}

func main() {
	defer fafaLog.Close()
	GetNovel(novel_url) // 爬取目录页 goquery过滤 写入通道
	if NovelLen < 1 {
		akaLog.WithFields(akaLog.Fields{
			"data": NovelLen,
		}).Fatal("资源出现了问题")

		os.Exit(2)
	}
	if bootRds := boot.BootRds(); bootRds != nil {
		panic(bootRds)
	}
	defer boot.AkaRds.Close()

	akaLog.Infof("获得了%d条小说章节 开始爬取\n", NovelLen)
	wg.Add(NovelLen)
	for i := 0; i < NovelLen; i++ {
		go GetNovelTask()
	}
	wg.Wait()
	akaLog.Info("爬取完毕...正在写入")

	rdsKeys, err := boot.AkaRds.ZRange(zaddForNovelsKey, 1, -1).Result()
	if err != nil {
		akaLog.Fatal("获取zset失败", err)
		os.Exit(2)
	}
	rdsKeyLen := len(rdsKeys) // 获取切片的长度

	if rdsKeyLen < 1 {
		akaLog.Fatalf("redis键数目不合法 数量为%d", rdsKeyLen)
		os.Exit(2)
	}
	wg.Add(rdsKeyLen)
	txtApdFileName := fmt.Sprintf("%s%s", zigzagPath, "/novels/")
	_, err = os.Stat(txtApdFileName)
	if err != nil {
		err := os.Mkdir(txtApdFileName, 0777)
		if err != nil {
			akaLog.Fatal(err)
			os.Exit(2)
		}
	}
	txtApdFileNameMix := fmt.Sprintf("%s%s.txt", txtApdFileName, NovelTitle)
	txtApdFile, err := os.OpenFile(txtApdFileNameMix, os.O_WRONLY|os.O_CREATE|os.O_APPEND, 0644)
	if err != nil {
		akaLog.Fatal(err)
		os.Exit(2)
	}
	defer txtApdFile.Close()

	//[]string range to boot groutine
	for _, novelMember := range rdsKeys {
		go fromRdsToFile(txtApdFile, novelMember)
	}
	boot.AkaRds.Del(zaddForNovelsKey)

	wg.Wait()
	akaLog.Infof("执行完毕,共记录小说%d章\n", rdsKeyLen)
	if zipOK {
		akaLog.Info("监测到需要压缩资源到ZIP...")
		tools.AkaCompressZip(zigzagPath) // 遍历文件夹下txt文件写入zip header
	}
}
