package tools

import (
	"fmt"
	"net/http"
)

func GetNovelData(novelUrl string) (*http.Response,error) {
	novel_response, left := http.Get(novelUrl)
	if left != nil {
		return novel_response,left
	}

	if novel_response.StatusCode != 200 {
		return novel_response,fmt.Errorf("http status code is %d and url is %s\n", novel_response.StatusCode, novelUrl)
	}
	return novel_response,nil
}
