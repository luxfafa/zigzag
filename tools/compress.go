package tools

import (
	"archive/zip"
	"fmt"
	"io"
	"io/ioutil"
	"os"
)

func AkaCompressZip(fPath string) {
	zipPath := fPath + "/example.zip"
	zipAka, err := os.Create(zipPath)
	if err != nil {
		panic(err)
	}
	defer zipAka.Close() // 新创建的ZIP
	zw := zip.NewWriter(zipAka)
	defer func() {
		// 检测一下是否成功关闭
		if err := zw.Close(); err != nil {
			fmt.Println(err)
		}
	}()
	txtPath := fPath + "/novels/noveltxt"
	readList, err := ioutil.ReadDir(txtPath) // readList []FileInfo

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
	fmt.Println("压缩完毕")
}
