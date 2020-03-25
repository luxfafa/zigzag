package tools

import (
	"strconv"
	"strings"
)

func UrlGetChapter(url string) float64 {

	urlSplit := strings.Split(url, "/")
	htmlString := urlSplit[len(urlSplit)-1]
	novelId := strings.Split(htmlString, ".")
	if len(novelId) == 2 {
		novelIdInt, err := strconv.Atoi(novelId[0])
		if err != nil {
			return 0
		}
		return float64(novelIdInt)
	}
	return 0
}
